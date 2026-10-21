package repository

import (
	"ordersystem/model"
	"time"
)

type DatabaseHandler struct {
	// drinks represent all available drinks
	drinks []model.Drink
	// orders serves as order history
	orders []model.Order
}

// todo
func NewDatabaseHandler() *DatabaseHandler {
	// Init the drinks slice with some test data
	// drinks := ...

	var drinks []model.Drink = []model.Drink{
		{
			ID:          12,
			Name:        "Beer",
			Price:       3.5,
			Description: "Cold",
		},
		{
			ID:          13,
			Name:        "Cola",
			Price:       2.8,
			Description: "Classic cola, 0.33l",
		},
		{
			ID:          14,
			Name:        "Water",
			Price:       1.5,
			Description: "Sparkling mineral water, 0.5l",
		},
		{
			ID:          15,
			Name:        "Coffee",
			Price:       2.2,
			Description: "Espresso, double shot",
		},
		{
			ID:          16,
			Name:        "Orange Juice",
			Price:       3.0,
			Description: "Freshly squeezed, 0.25l",
		},
	}

	// Init orders slice with some test data
	orders := []model.Order{
		{
			DrinkID:   12,
			CreatedAt: time.Now().Add(-2 * time.Hour),
			Amount:    2,
		},
		{
			DrinkID:   13,
			CreatedAt: time.Now().Add(-90 * time.Minute),
			Amount:    1,
		},
		{
			DrinkID:   12,
			CreatedAt: time.Now().Add(-1 * time.Hour),
			Amount:    3,
		},
		{
			DrinkID:   15,
			CreatedAt: time.Now().Add(-30 * time.Minute),
			Amount:    1,
		},
	}

	return &DatabaseHandler{
		drinks: drinks,
		orders: orders,
	}
}

func (db *DatabaseHandler) GetDrinks() []model.Drink {
	return db.drinks
}

func (db *DatabaseHandler) GetOrders() []model.Order {
	return db.orders
}

// todo
func (db *DatabaseHandler) GetTotalledOrders() map[uint64]uint64 {
	// calculate total orders
	// key = DrinkID, value = Amount of orders
	// totalledOrders map[uint64]uint64

	totalledOrders := make(map[uint64]uint64)

	for _, order := range db.orders {
		totalledOrders[order.DrinkID] += order.Amount
	}

	return totalledOrders
}

func (db *DatabaseHandler) AddOrder(order *model.Order) {
	// todo
	// add order to db.orders slice

	db.orders = append(db.orders, *order)
}
