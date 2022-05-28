package db

import "github.com/jmoiron/sqlx"

type query struct {
	tx    *sqlx.Tx
	model interface{}
}

func (q *query) Where(wh string) *query {
	return q
}

func (q *query) Join(table, wh string) *query {
	return q
}

func (q *query) Get(result interface{}) error {
	return nil
}

func (q *query) List(results interface{}) error {
	return nil
}

func Query(tx *sqlx.Tx, model interface{}) *query {
	return &query{model: model}
}

type exec struct {
	tx    *sqlx.Tx
	model interface{}
}

func (e *exec) Save(record interface{}) error {
	return nil
}

func (e *exec) Delete(record interface{}) error {
	return nil
}

func Exec(tx *sqlx.Tx, model interface{}) *exec {
	return &exec{model: model}
}
