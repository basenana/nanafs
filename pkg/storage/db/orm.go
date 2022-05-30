package db

import "github.com/jmoiron/sqlx"

type Table interface {
	TableName() string
}

type query struct {
	tx    *sqlx.Tx
	model Table
}

func (q *query) Where(wh string) *query {
	return q
}

func (q *query) Join(t Table, wh string) *query {
	return q
}

func (q *query) Get(result interface{}) error {
	return nil
}

func (q *query) List(results interface{}) error {
	return nil
}

func Query(tx *sqlx.Tx, model Table) *query {
	return &query{model: model}
}

type exec struct {
	tx    *sqlx.Tx
	model Table
}

func (e *exec) Save() error {
	return nil
}

func (e *exec) Delete() error {
	return nil
}

func Exec(tx *sqlx.Tx, model Table) *exec {
	return &exec{model: model}
}
