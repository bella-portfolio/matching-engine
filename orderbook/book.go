package orderbook

import (
	"container/heap"
	"sync"
	"time"
)

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

type OrderType string

const (
	Limit  OrderType = "LIMIT"
	Market OrderType = "MARKET"
)

type OrderStatus string

const (
	Open      OrderStatus = "OPEN"
	Filled    OrderStatus = "FILLED"
	Partial   OrderStatus = "PARTIAL"
	Cancelled OrderStatus = "CANCELLED"
)

type Order struct {
	ID        string      `json:"id"`
	Side      Side        `json:"side"`
	Type      OrderType   `json:"type"`
	Price     float64     `json:"price"`
	Quantity  float64     `json:"quantity"`
	FilledQty float64     `json:"filled_qty"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

type Trade struct {
	ID        string    `json:"id"`
	BuyID     string    `json:"buy_id"`
	SellID    string    `json:"sell_id"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
}

func (o *Order) remaining() float64 {
	return o.Quantity - o.FilledQty
}

// OrderBook is a thread-safe price-time priority matching engine.
type OrderBook struct {
	mu        sync.RWMutex
	buyHeap   *orderHeap
	sellHeap  *orderHeap
	orders    map[string]*Order
	trades    []Trade
	maxTrades int
	tradeSeq  int64
	orderSeq  int64
}

func New(maxTrades int) *OrderBook {
	buyH := newOrderHeap(Buy)
	sellH := newOrderHeap(Sell)
	heap.Init(buyH)
	heap.Init(sellH)

	return &OrderBook{
		buyHeap:   buyH,
		sellHeap:  sellH,
		orders:    make(map[string]*Order),
		trades:    make([]Trade, 0, maxTrades),
		maxTrades: maxTrades,
	}
}

// PlaceOrder submits an order and immediately attempts to match.
// Returns the placed order and any resulting trades.
func (ob *OrderBook) PlaceOrder(o *Order) ([]Trade, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.orderSeq++
	o.ID = generateID(ob.orderSeq)
	o.CreatedAt = time.Now()
	o.Status = Open
	ob.orders[o.ID] = o

	var trades []Trade

	if o.Side == Buy {
		trades = ob.matchBuy(o)
	} else {
		trades = ob.matchSell(o)
	}

	return trades, nil
}

// CancelOrder cancels an open order.
func (ob *OrderBook) CancelOrder(id string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	o, ok := ob.orders[id]
	if !ok || o.Status != Open {
		return false
	}
	o.Status = Cancelled
	return true
}

// GetOrder returns an order by ID.
func (ob *OrderBook) GetOrder(id string) *Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.orders[id]
}

// GetTrades returns the most recent trades.
func (ob *OrderBook) GetTrades(limit int) []Trade {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if limit <= 0 || limit > len(ob.trades) {
		limit = len(ob.trades)
	}
	start := len(ob.trades) - limit
	result := make([]Trade, limit)
	copy(result, ob.trades[start:])
	return result
}

// BookSnapshot returns current order book depth.
func (ob *OrderBook) BookSnapshot() (bids, asks []PriceLevel) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return ob.buyHeap.snapshot(), ob.sellHeap.snapshot()
}

// matchBuy matches a buy order against sell orders.
func (ob *OrderBook) matchBuy(o *Order) []Trade {
	var trades []Trade

	for o.remaining() > 0 && ob.sellHeap.Len() > 0 {
		top := ob.sellHeap.Peek()
		if o.Type == Limit && o.Price < top.Price {
			break
		}

		matchQty := min(o.remaining(), top.remaining())
		matchPrice := top.Price

		top.FilledQty += matchQty
		o.FilledQty += matchQty

		if top.remaining() <= 0 {
			top.Status = Filled
			heap.Pop(ob.sellHeap)
		} else {
			top.Status = Partial
		}

		ob.tradeSeq++
		trade := Trade{
			ID:        generateTradeID(ob.tradeSeq),
			BuyID:     o.ID,
			SellID:    top.ID,
			Price:     matchPrice,
			Quantity:  matchQty,
			Timestamp: time.Now(),
		}
		trades = append(trades, trade)
		ob.addTrade(trade)
	}

	if o.remaining() <= 0 {
		o.Status = Filled
	} else if o.FilledQty > 0 {
		o.Status = Partial
		heap.Push(ob.buyHeap, o)
	} else {
		heap.Push(ob.buyHeap, o)
	}

	return trades
}

// matchSell matches a sell order against buy orders.
func (ob *OrderBook) matchSell(o *Order) []Trade {
	var trades []Trade

	for o.remaining() > 0 && ob.buyHeap.Len() > 0 {
		top := ob.buyHeap.Peek()
		if o.Type == Limit && o.Price > top.Price {
			break
		}

		matchQty := min(o.remaining(), top.remaining())
		matchPrice := top.Price

		top.FilledQty += matchQty
		o.FilledQty += matchQty

		if top.remaining() <= 0 {
			top.Status = Filled
			heap.Pop(ob.buyHeap)
		} else {
			top.Status = Partial
		}

		ob.tradeSeq++
		trade := Trade{
			ID:        generateTradeID(ob.tradeSeq),
			BuyID:     top.ID,
			SellID:    o.ID,
			Price:     matchPrice,
			Quantity:  matchQty,
			Timestamp: time.Now(),
		}
		trades = append(trades, trade)
		ob.addTrade(trade)
	}

	if o.remaining() <= 0 {
		o.Status = Filled
	} else if o.FilledQty > 0 {
		o.Status = Partial
		heap.Push(ob.sellHeap, o)
	} else {
		heap.Push(ob.sellHeap, o)
	}

	return trades
}

func (ob *OrderBook) addTrade(t Trade) {
	ob.trades = append(ob.trades, t)
	if len(ob.trades) > ob.maxTrades {
		ob.trades = ob.trades[len(ob.trades)-ob.maxTrades:]
	}
}
