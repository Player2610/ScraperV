package orders

import "errors"

// ErrForbidden is returned when a user tries to access another user's order.
var ErrForbidden = errors.New("forbidden")

// ErrOutsideZone is returned when the delivery address is outside the coverage area.
var ErrOutsideZone = errors.New("dirección fuera de zona de cobertura")

// ErrEmptyCart is returned when trying to checkout with an empty cart.
var ErrEmptyCart = errors.New("carrito vacío")

// ErrUnavailableItems is returned when the cart contains unavailable items.
var ErrUnavailableItems = errors.New("el carrito contiene productos no disponibles")

// ErrNotFound is returned when an order is not found.
var ErrNotFound = errors.New("not found")
