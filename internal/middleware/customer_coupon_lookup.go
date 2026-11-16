package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/utils"
)

const (
	customerCouponLookupWindow       = time.Minute
	customerCouponLookupIPLimit      = 30
	customerCouponLookupPhoneLimit   = 5
	customerCouponLookupMaxBuckets   = 10000
	customerCouponLookupBodyMaxBytes = 1024
	customerCouponLookupSweepEvery   = 5 * time.Second
)

type customerCouponLookupRequest struct {
	Phone string `json:"phone"`
}

type customerCouponLookupBucket struct {
	count     int
	expiresAt time.Time
}

type customerCouponLookupLimiter struct {
	secret      []byte
	ipLimit     int
	phoneLimit  int
	maxBuckets  int
	window      time.Duration
	mu          sync.Mutex
	buckets     map[string]customerCouponLookupBucket
	lastSweepAt time.Time
}

func CustomerCouponLookupRateLimit(secret string) gin.HandlerFunc {
	limiter, ready := newCustomerCouponLookupLimiter(
		secret,
		customerCouponLookupIPLimit,
		customerCouponLookupPhoneLimit,
		customerCouponLookupMaxBuckets,
		customerCouponLookupWindow,
	)
	return customerCouponLookupProtection(limiter, ready)
}

func customerCouponLookupProtection(limiter *customerCouponLookupLimiter, ready bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ready {
			utils.Error(c, http.StatusServiceUnavailable, "coupon lookup temporarily unavailable")
			c.Abort()
			return
		}

		now := time.Now()
		peerIP, ok := customerCouponLookupPeerIP(c.Request.RemoteAddr)
		if !ok {
			utils.Error(c, http.StatusBadRequest, "invalid request peer address")
			c.Abort()
			return
		}
		if !limiter.take("ip", peerIP, limiter.ipLimit, now) {
			utils.Error(c, http.StatusTooManyRequests, "too many coupon lookup requests")
			c.Abort()
			return
		}

		if _, hasPhoneQuery := c.Request.URL.Query()["phone"]; hasPhoneQuery {
			utils.Error(c, http.StatusBadRequest, "send phone in the JSON request body")
			c.Abort()
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, customerCouponLookupBodyMaxBytes))
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "invalid coupon lookup request")
			c.Abort()
			return
		}

		var request customerCouponLookupRequest
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			utils.Error(c, http.StatusBadRequest, "invalid coupon lookup request")
			c.Abort()
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			utils.Error(c, http.StatusBadRequest, "invalid coupon lookup request")
			c.Abort()
			return
		}

		phone := strings.TrimSpace(request.Phone)
		if len(phone) != 11 || phone[0] != '1' {
			utils.Error(c, http.StatusBadRequest, "手机号需要是 11 位数字")
			c.Abort()
			return
		}
		for _, digit := range phone[1:] {
			if digit < '0' || digit > '9' {
				utils.Error(c, http.StatusBadRequest, "手机号需要是 11 位数字")
				c.Abort()
				return
			}
		}

		if !limiter.take("phone", phone, limiter.phoneLimit, now) {
			utils.Error(c, http.StatusTooManyRequests, "too many coupon lookup requests")
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

func customerCouponLookupPeerIP(remoteAddr string) (string, bool) {
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "", false
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}

func newCustomerCouponLookupLimiter(secret string, ipLimit, phoneLimit, maxBuckets int, window time.Duration) (*customerCouponLookupLimiter, bool) {
	normalizedSecret := strings.ToLower(strings.TrimSpace(secret))
	if len(secret) < 32 || strings.Contains(normalizedSecret, "replace-with") || strings.Contains(normalizedSecret, "your-production") {
		return nil, false
	}
	if ipLimit < 1 || phoneLimit < 1 || maxBuckets < 1 || window <= 0 {
		return nil, false
	}
	return &customerCouponLookupLimiter{
		secret:     []byte(secret),
		ipLimit:    ipLimit,
		phoneLimit: phoneLimit,
		maxBuckets: maxBuckets,
		window:     window,
		buckets:    make(map[string]customerCouponLookupBucket),
	}, true
}

func (l *customerCouponLookupLimiter) take(dimension, value string, limit int, now time.Time) bool {
	key := l.bucketKey(dimension, value)
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastSweepAt.IsZero() || now.Sub(l.lastSweepAt) >= customerCouponLookupSweepEvery {
		for oldKey, oldBucket := range l.buckets {
			if !now.Before(oldBucket.expiresAt) {
				delete(l.buckets, oldKey)
			}
		}
		l.lastSweepAt = now
	}

	bucket, exists := l.buckets[key]
	if exists && !now.Before(bucket.expiresAt) {
		bucket = customerCouponLookupBucket{expiresAt: now.Add(l.window)}
		bucket.count++
		l.buckets[key] = bucket
		return bucket.count <= limit
	}
	if !exists {
		if len(l.buckets) >= l.maxBuckets {
			return false
		}
		bucket = customerCouponLookupBucket{expiresAt: now.Add(l.window)}
	}

	bucket.count++
	l.buckets[key] = bucket
	return bucket.count <= limit
}

func (l *customerCouponLookupLimiter) bucketKey(dimension, value string) string {
	mac := hmac.New(sha256.New, l.secret)
	_, _ = mac.Write([]byte("customer-coupon-lookup-v1\x00"))
	_, _ = mac.Write([]byte(dimension))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return dimension + ":" + hex.EncodeToString(mac.Sum(nil))
}
