package rest

import (
	"encoding/json"
	"net/http"
	"ordersystem/model"
	"ordersystem/repository"

	"github.com/go-chi/render"
)

// GetMenu 			godoc
// @tags 			Menu
// @Description 	Returns the menu of all drinks
// @Produce  		json
// @Success 		200 {array} model.Drink
// @Router 			/api/menu [get]
func GetMenu(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// todo
		// get slice from db
		// render.Status(r, http.StatusOK)
		// render.JSON(w, r, <your-slice>)

		drinks := db.GetDrinks()
		render.Status(r, http.StatusOK)
		render.JSON(w, r, drinks)

	}
}

// todo create GetOrders /api/order/all

// GetOrders            godoc
// @tags                        Order
// @Description         Returns all orders
// @Produce             json
// @Success             200 {array} model.Order
// @Router                      /api/order/all [get]
func GetOrders(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orders := db.GetOrders()
		render.Status(r, http.StatusOK)
		render.JSON(w, r, orders)
	}
}

// todo create GetOrdersTotal /api/order/total

// GetOrdersTotal       godoc
// @tags                        Order
// @Description         Returns total amount ordered per drink (key = DrinkID, value = Amount)
// @Produce             json
// @Success             200 {object} map[uint64]uint64
// @Router                      /api/order/totalled [get]
func GetOrdersTotal(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orders := db.GetTotalledOrders()
		render.Status(r, http.StatusOK)
		render.JSON(w, r, orders)
	}
}

// PostOrder 		godoc
// @tags 			Order
// @Description 	Adds an order to the db
// @Accept 			json
// @Param 			b body model.Order true "Order"
// @Produce  		json
// @Success 		200
// @Failure     	400
// @Router 			/api/order [post]
func PostOrder(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// todo
		// declare empty order struct
		var order model.Order

		// err := json.NewDecoder(r.Body).Decode(&<your-order-struct>)
		err := json.NewDecoder(r.Body).Decode(&order)

		// handle error and render Status 400
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, err.Error())
			return
		}

		// add to db
		db.AddOrder(&order)

		render.Status(r, http.StatusOK)
		render.JSON(w, r, "ok")
	}
}
