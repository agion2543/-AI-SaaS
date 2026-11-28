// Merchant onboarding review (P3-1): a merchant is pending from registration
// until a super-admin approves it. A rejection keeps status=pending with a
// non-empty review_reason and a non-NULL reviewed_at; resubmission clears
// all three columns back to a clean pending state.
export const isMerchantPending = (merchant = {}) => merchant?.status === 'pending'

export const isMerchantOnboardingRejected = (merchant = {}) =>
  merchant?.status === 'pending' &&
  String(merchant?.review_reason || '').trim() !== '' &&
  merchant?.reviewed_at !== null &&
  merchant?.reviewed_at !== undefined &&
  merchant?.reviewed_at !== ''
