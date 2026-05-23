package orderbook

type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Orders   int     `json:"orders"`
}

// orderHeap implements heap.Interface with price-time priority.
type orderHeap struct {
	orders []*Order
	side   Side
}

func newOrderHeap(side Side) *orderHeap {
	return &orderHeap{side: side, orders: make([]*Order, 0)}
}

func (h *orderHeap) Len() int { return len(h.orders) }

func (h *orderHeap) Less(i, j int) bool {
	a, b := h.orders[i], h.orders[j]
	if h.side == Buy {
		if a.Price != b.Price {
			return a.Price > b.Price // Higher price first
		}
	} else {
		if a.Price != b.Price {
			return a.Price < b.Price // Lower price first
		}
	}
	return a.CreatedAt.Before(b.CreatedAt) // Same price: FIFO
}

func (h *orderHeap) Swap(i, j int) {
	h.orders[i], h.orders[j] = h.orders[j], h.orders[i]
}

func (h *orderHeap) Push(x any) {
	h.orders = append(h.orders, x.(*Order))
}

func (h *orderHeap) Pop() any {
	n := len(h.orders)
	o := h.orders[n-1]
	h.orders = h.orders[:n-1]
	return o
}

func (h *orderHeap) Peek() *Order {
	if len(h.orders) == 0 {
		return nil
	}
	return h.orders[0]
}

// snapshot aggregates open orders into price levels.
func (h *orderHeap) snapshot() []PriceLevel {
	if len(h.orders) == 0 {
		return nil
	}

	levels := make(map[float64]*PriceLevel)
	var prices []float64

	for _, o := range h.orders {
		if o.Status == Open || o.Status == Partial {
			if _, ok := levels[o.Price]; !ok {
				levels[o.Price] = &PriceLevel{Price: o.Price}
				prices = append(prices, o.Price)
			}
			levels[o.Price].Quantity += o.remaining()
			levels[o.Price].Orders++
		}
	}

	result := make([]PriceLevel, 0, len(prices))
	for _, p := range prices {
		result = append(result, *levels[p])
	}
	return result
}
