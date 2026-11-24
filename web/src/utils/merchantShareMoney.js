export const shareConfigAmountYuan = (amountFen, fallbackFen) => (
  Number(((amountFen ?? fallbackFen ?? 0) / 100).toFixed(2))
)
