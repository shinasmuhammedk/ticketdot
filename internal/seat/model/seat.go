package model

type Seat struct {
	SeatNumber string `db:"seat_number" json:"seat_number"`
	SeatType   string `db:"seat_type" json:"seat_type"`
	DeckType   string `db:"deck_type" json:"deck_type"`
	RowNum     int    `db:"row_num" json:"row_num"`
	ColNum     int    `db:"col_num" json:"col_num"`
	IsWindow   bool   `db:"is_window" json:"is_window"`
	Status     string `db:"status" json:"status"`
}