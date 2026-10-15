package repository

import "ticketdot/internal/search/model"

type SearchRepository interface {
	SearchBuses(from string, to string, date string) ([]model.BusSearchResult, error)
	GetScheduleDetails(scheduleID string) (*model.ScheduleDetailsResponse, error)
	GetCities() ([]model.City, error)
}
