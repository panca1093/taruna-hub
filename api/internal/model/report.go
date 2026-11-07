package model

import "time"

// RevenueBucket is a count and an exact decimal total, kept as the string PostgreSQL produced.
type RevenueBucket struct {
	Key   string
	Count int
	Total string
}

// RevenueDay is one Asia/Jakarta calendar day of the recap.
type RevenueDay struct {
	Date        time.Time
	Orders      int
	OrdersTotal string
	Topups      int
	TopupsTotal string
}

// RevenueRecap summarises confirmed orders and approved top-ups of one tenant for an inclusive range of days.
type RevenueRecap struct {
	From, To  time.Time
	Orders    RevenueBucket
	ByMethod  []RevenueBucket
	ByProduct []RevenueBucket
	Topups    RevenueBucket
	Days      []RevenueDay
	DaysTotal int
}
