// Copyright 2018-20 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package dataframe

import (
	"context"

	"golang.org/x/exp/rand"
)

// Options sets various options.
type Options struct {

	// Don't apply lock.
	DontLock bool
}

// IsEqualOptions sets various options for the IsEqual function.
type IsEqualOptions struct {

	// Don't apply lock.
	DontLock bool

	// Check if name is the same.
	CheckName bool
}

// NilCountOptions sets various options for the NilCount function.
type NilCountOptions struct {

	// Ctx adds a context.
	Ctx context.Context

	// R is used to limit the range.
	R *Range

	// Don't apply lock.
	DontLock bool

	// When StopAtOneNil is set, the function will return after finding at least 1 nil value.
	// It can be used as a ContainsNil function when used with R.
	StopAtOneNil bool
}

// ValueToStringFormatter is used to convert a value
// into a string. Val can be nil or the concrete
// type stored by the series.
type ValueToStringFormatter func(val interface{}) string

// Series is a collection of data that could be of any type.
// It is usually used with DataFrame.
type Series interface {

	// Name returns the series name.
	Name(opts ...Options) string

	// Rename renames the series.
	Rename(n string, opts ...Options)

	// Type returns the type of data the series holds.
	Type() string

	// NRows returns how many rows the series contains.
	NRows(opts ...Options) int

	// Value returns the value of a particular row.
	// The return value could be nil or the concrete type
	// the data type held by the series.
	// Pointers are never returned.
	Value(row int, opts ...Options) interface{}

	// ValueString returns a string representation of a
	// particular row. The string representation is defined
	// by the function set in SetValueToStringFormatter.
	// By default, a nil value is returned as "NaN".
	ValueString(row int, opts ...Options) string

	// Prepend is used to set a value to the beginning of the
	// series. val can be a concrete data type or nil. Nil
	// represents the absence of a value.
	Prepend(val interface{}, opts ...Options)

	// Append is used to set a value to the end of the series.
	// val can be a concrete data type or nil. Nil represents
	// the absence of a value.
	Append(val interface{}, opts ...Options) int

	// Insert is used to set a value at an arbitrary row in
	// the series. All existing values from that row onwards
	// are shifted by 1. val can be a concrete data type or nil.
	// Nil represents the absence of a value.
	Insert(row int, val interface{}, opts ...Options)

	// Remove is used to delete the value of a particular row.
	Remove(row int, opts ...Options)

	// Reset is used clear all data contained in the Series.
	Reset(opts ...Options)

	// ValuesIterator will return a function that can be used to iterate through all the values.
	ValuesIterator(opts ...ValuesOptions) func() (*int, interface{}, int)

	// Update is used to update the value of a particular row.
	// val can be a concrete data type or nil. Nil represents
	// the absence of a value.
	Update(row int, val interface{}, opts ...Options)

	// SetValueToStringFormatter is used to set a function
	// to convert the value of a particular row to a string
	// representation.
	SetValueToStringFormatter(f ValueToStringFormatter)

	// Sort will sort the series.
	// It will return true if sorting was completed or false when the context is canceled.
	Sort(ctx context.Context, opts ...SortOptions) (completed bool)

	// IsEqualFunc returns true if a is equal to b.
	IsEqualFunc(a, b interface{}) bool

	// IsLessThanFunc returns true if a is less than b.
	IsLessThanFunc(a, b interface{}) bool

	// Swap is used to swap 2 values based on their row position.
	Swap(row1, row2 int, opts ...Options)

	// Lock will lock the Series allowing you to directly manipulate
	// the underlying slice with confidence.
	Lock()

	// Unlock will unlock the Series that was previously locked.
	Unlock()

	// Copy will create a new copy of the series.
	// It is recommended that you lock the Series before attempting
	// to Copy.
	Copy(r ...Range) Series

	// ContainsNil will return whether or not the series contains any nil values.
	ContainsNil(opts ...Options) bool

	// NilCount will return how many nil values are in the series.
	NilCount(opts ...NilCountOptions) (int, error)

	// IsEqual returns true if s2's values are equal to s.
	IsEqual(ctx context.Context, s2 Series, opts ...IsEqualOptions) (bool, error)
}

// NewSerieser is an interface for a Series to create a new initialized Series of the same type.
type NewSerieser interface {

	// NewSeries creates a new initialized Series of the same type.
	NewSeries(name string, init *SeriesInit) Series
}

// Rander is an interface for generating random float64.
//
// See: https://godoc.org/golang.org/x/exp/rand for a random generator source.
// See: https://godoc.org/gonum.org/v1/gonum/stat/distuv for various random distributions.
type Rander interface {

	// Rand returns a randomly generated float64.
	Rand() float64
}

// FillRandOptions configures how FillRand should behave.
type FillRandOptions struct {

	// R is used to only randomly fill a range of rows.
	R *Range

	// Extra is used to pass extra custom data.
	Extra interface{}
}

// FillRander is an interface for generating a Series with random values.
type FillRander interface {

	// FillRand will fill a Series with random data. probNil is a value between between 0 and 1 which
	// determines if a row is given a nil value.
	//
	// Example:
	//
	//  import "golang.org/x/exp/rand"
	//  import "gonum.org/v1/gonum/stat/distuv"
	//  import "time"
	//
	//  src := rand.NewSource(uint64(time.Now().UTC().UnixNano()))
	//  uniform := distuv.Uniform{Min: 0, Max: 10000, Src: src}
	//  s.FillRand(src, 0.5, uniform)
	//
	FillRand(src rand.Source, probNil float64, rander Rander, opts ...FillRandOptions)
}
