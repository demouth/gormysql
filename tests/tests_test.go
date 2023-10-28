package gormysql_test

import (
	"fmt"
	"testing"

	"github.com/demouth/gormysql"
)

var (
	db gormysql.DB
)

func init() {
	var err error
	db, err = gormysql.Open("gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		panic(fmt.Sprintf("No error should happen when connect database, but got %+v", err))
	}
}

func TestSample(t *testing.T) {
}
