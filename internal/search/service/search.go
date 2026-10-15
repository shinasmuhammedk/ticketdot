package service

import (
	"errors"
	"strings"

	"ticketdot/internal/search/model"
	"ticketdot/internal/search/repository"
)

type SearchService struct {
	repo repository.SearchRepository
}

func NewSearchService(repo repository.SearchRepository) *SearchService {
	return &SearchService{
		repo: repo,
	}
}

func (s *SearchService) SearchBuses(from, to, date string) ([]model.BusSearchResult, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	date = strings.TrimSpace(date)

	if from == "" {
		return nil, errors.New("from city is required")
	}

	if to == "" {
		return nil, errors.New("to city is required")
	}

	if date == "" {
		return nil, errors.New("date is required")
	}

	return s.repo.SearchBuses(from, to, date)
}

func (s *SearchService) GetScheduleDetails(scheduleID string) (*model.ScheduleDetailsResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, errors.New("schedule id is required")
	}

	return s.repo.GetScheduleDetails(scheduleID)
}

func (s *SearchService) GetCities() ([]model.City, error) {
	return s.repo.GetCities()
}

