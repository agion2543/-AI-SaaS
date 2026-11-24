export const successfulRefundsOnDate = (refunds, date) => (
  (Array.isArray(refunds) ? refunds : []).filter((refund) => (
    refund?.status === 'success' && String(refund.created_at || '').slice(0, 10) === date
  ))
)
