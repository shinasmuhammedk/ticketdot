package repository

import "ticketdot/internal/seat/model"

type SeatRepository interface {
	GetSeatsByScheduleID(scheduleID string) ([]model.Seat, error)
}
