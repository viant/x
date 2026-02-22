package orders

import "github.com/viant/x/examples/namespace_files/customer"

// Order references a type from another package to demonstrate per-package files.
type Order struct {
	Number string
	Buyer  customer.Customer
}
