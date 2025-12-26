package models

type Port struct {
	Number   string `bson:"number" json:"number"`
	Protocol string `bson:"protocol" json:"protocol"`
	State    string `bson:"state" json:"state"`
	Service  string `bson:"service" json:"service"`
}
