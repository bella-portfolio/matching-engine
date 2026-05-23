package orderbook

import "fmt"

func generateID(seq int64) string {
	return fmt.Sprintf("ORD-%06d", seq)
}

func generateTradeID(seq int64) string {
	return fmt.Sprintf("TRD-%06d", seq)
}
