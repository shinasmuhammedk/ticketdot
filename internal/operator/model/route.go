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

type RouteResponse struct {
	ID          string `db:"id" json:"id"`
	Origin      string `db:"origin" json:"origin"`
	Destination string `db:"destination" json:"destination"`
}
