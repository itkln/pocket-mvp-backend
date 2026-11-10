// Package money holds the one type every module uses for amounts.
package money

// Minor is an amount in the currency's minor units (cents for EUR). Amounts are never floats:
// tips, taxes and totals are computed in Minor and rounded explicitly where a percentage applies.
// It is stored in bigint/integer columns and sent to clients as a JSON number.
type Minor int64

// Times is the amount for quantity units, such as a line total.
func (m Minor) Times(quantity int) Minor { return m * Minor(quantity) }
