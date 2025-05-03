package models

type OrderAccrual struct {
	UserID      int         `json:"omitempty"`
	OrderNumber string      `json:"order"`
	Status      OrderStatus `json:"status"`
	Accrual     float64     `json:"accrual,omitempty"`
}
