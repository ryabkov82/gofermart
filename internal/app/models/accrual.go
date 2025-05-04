package models

type OrderAccrual struct {
	UserID      int         `json:"omitempty"`
	OrderNumber string      `json:"order"`
	Status      OrderStatus `json:"status"`
	StatusOld   OrderStatus `json:"status_old"`
	Accrual     float64     `json:"accrual,omitempty"`
}
