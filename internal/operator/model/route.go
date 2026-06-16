package model

type CreateRouteRequest struct {
	OriginCityID      string `json:"origin_city_id"`
	DestinationCityID string `json:"destination_city_id"`
}

type CreateRouteResponse struct {
	ID                string `json:"id"`
	OriginCityID      string `json:"origin_city_id"`
	DestinationCityID string `json:"destination_city_id"`
}