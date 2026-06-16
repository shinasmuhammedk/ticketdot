package service

import (
	"errors"
	"strings"

	"ticketdot/internal/seat/model"
	"ticketdot/internal/seat/repository"
)

type SeatService struct {
	repo repository.SeatRepository
}

func NewSeatService(repo repository.SeatRepository) *SeatService {
	return &SeatService{
		repo: repo,
	}
}

func (s *SeatService) GetSeatsByScheduleID(scheduleID string) ([]model.Seat, error) {
	scheduleID = strings.TrimSpace(scheduleID)

	if scheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	return s.repo.GetSeatsByScheduleID(scheduleID)
}