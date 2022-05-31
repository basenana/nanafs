package db

import (
	"bytes"
	"fmt"
	"github.com/jmoiron/sqlx"
	"reflect"
	"strings"
)

type Table interface {
	TableName() string
}

type query struct {
	tx     *sqlx.Tx
	model  Table
	joined Table
	joinOn string
	where  []matching
}

func (q *query) Where(col, op string, val interface{}) *query {
	q.where = append(q.where, matching{
		col: col,
		op:  op,
		val: val,
	})
	return q
}

func (q *query) Join(t Table, wh string) *query {
	q.joined = t
	q.joinOn = wh
	return q
}

func (q *query) Get(result interface{}) error {
	cols := listColumns(q.model)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("SELECT %s FROM %s ", strings.Join(cols, ","), q.model.TableName()))
	var args []interface{}
	if len(q.where) > 0 {
		buf.WriteString("WHERE")
		for i, m := range q.where {
			buf.WriteString(fmt.Sprintf(" %s%s$%d ", m.col, m.op, i+1))
			if i+1 < len(q.where) {
				buf.WriteString("AND")
			}
			args = append(args, m.val)
		}
	}
	return q.tx.Get(result, buf.String(), args...)
}

func (q *query) List(results interface{}) error {
	cols := listColumns(q.model)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("SELECT %s FROM %s ", strings.Join(cols, ","), q.model.TableName()))
	var args []interface{}
	if len(q.where) > 0 {
		buf.WriteString("WHERE")
		for i, m := range q.where {
			buf.WriteString(fmt.Sprintf(" %s%s$%d ", m.col, m.op, i+1))
			if i+1 < len(q.where) {
				buf.WriteString("AND")
			}
			args = append(args, m.val)
		}
	}
	return q.tx.Select(results, buf.String(), args...)
}

func Query(tx *sqlx.Tx, model Table) *query {
	return &query{tx: tx, model: model}
}

type exec struct {
	tx    *sqlx.Tx
	model Table
}

func (e *exec) Insert() error {
	cols := listColumns(e.model)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (:%s)",
		e.model.TableName(), strings.Join(cols, ","), strings.Join(cols, ",:")))
	_, err := e.tx.NamedExec(buf.String(), e.model)
	return err
}

func (e *exec) Update() error {
	cols := listColumns(e.model)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("UPDATE %s SET ", e.model.TableName()))
	for i, col := range cols {
		buf.WriteString(fmt.Sprintf("%s=:%s", col, col))
		if i+1 < len(cols) {
			buf.WriteString(",")
		}
	}
	buf.WriteString(fmt.Sprintf(" WHERE id=:id"))
	_, err := e.tx.NamedExec(buf.String(), e.model)
	return err
}

func (e *exec) Delete() error {
	q := fmt.Sprintf("DELETE FROM %s WHERE id=:id", e.model.TableName())
	_, err := e.tx.NamedExec(q, e.model)
	return err
}

func Exec(tx *sqlx.Tx, model Table) *exec {
	return &exec{tx: tx, model: model}
}

type matching struct {
	col string
	op  string
	val interface{}
}

func listColumns(m Table) (result []string) {
	v := reflect.ValueOf(m)
	i := reflect.Indirect(v)
	s := i.Type()
	for i := 0; i < s.NumField(); i++ {
		field := s.Field(i)

		// Get the field tag value
		tag := field.Tag.Get("db")
		result = append(result, tag)
	}
	return
}
