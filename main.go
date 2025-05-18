package gormysql

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type (
	DB struct {
		db *sql.DB
	}
	Chain struct {
		db     *sql.DB
		Errors []error
		Error  error
		value  any

		whereClause []map[string]any
		orderStrs   []string
	}
	Do struct {
		db        *sql.DB
		chain     *Chain
		sqlResult sql.Result
		Errors    []error
		model     *Model
		value     any
		sql       string
		sqlVars   []any

		whereClause []map[string]any
		orderStrs   []string
		limitStr    string
	}
	Model struct {
		data any
	}
	Field struct {
		Name           string
		Value          any
		SqlType        string
		DbName         string
		AutoCreateTime bool
		AutoUpdateTime bool
		IsPrimaryKey   bool
	}
)

func Open(source string) (db DB, err error) {
	db.db, err = sql.Open("mysql", source)
	return
}

func OpenWithDriver(driverName, source string) (db DB, err error) {
	db.db, err = sql.Open(driverName, source)
	return
}

func (db *DB) Exec(sql string) *Chain {
	return db.buildChanin().Exec(sql)
}

func (db *DB) CreateTable(value any) *Chain {
	return db.buildChanin().CreateTable(value)
}

func (db *DB) Where(queryString any, args ...any) *Chain {
	return db.buildChanin().Where(queryString, args...)
}

func (db *DB) First(out any, where ...any) *Chain {
	return db.buildChanin().First(out, where...)
}

func (db *DB) Find(out any, where ...any) *Chain {
	return db.buildChanin().Find(out, where...)
}

func (db *DB) Save(value any) *Chain {
	return db.buildChanin().Save(value)
}

func (db *DB) Delete(value any) *Chain {
	return db.buildChanin().Delete(value)
}

func (db *DB) buildChanin() *Chain {
	return &Chain{db: db.db}
}

//--------- Chain ---------

func (c *Chain) Exec(sql string) *Chain {
	c.do(nil).exec(sql)
	return c
}

func (c *Chain) CreateTable(value any) *Chain {
	c.do(value).createTable().exec()
	return c
}

func (c *Chain) Where(queryString any, args ...any) *Chain {
	c.whereClause = append(c.whereClause, map[string]any{
		"query": queryString,
		"args":  args,
	})
	return c
}

func (c *Chain) Order(value string) *Chain {
	c.orderStrs = append(c.orderStrs, value)
	return c
}

func (c *Chain) First(out any, where ...any) *Chain {
	do := c.do(out)
	do.limitStr = "1"
	do.query(where...)
	return c
}

func (c *Chain) Find(out any, where ...any) *Chain {
	c.do(out).query(where...)
	return c
}

func (c *Chain) Save(value any) *Chain {
	c.do(value).save()
	return c
}

func (c *Chain) Delete(value any) *Chain {
	c.do(value).delete()
	return c
}

func (c *Chain) do(value any) *Do {
	var do Do
	do.db = c.db
	do.chain = c
	do.whereClause = c.whereClause
	do.orderStrs = c.orderStrs

	c.value = value
	do.setModel(value)
	return &do
}

func (c *Chain) err(err error) error {
	if err != nil {
		c.Errors = append(c.Errors, err)
		c.Error = err
	}
	return err
}

//--------- Do ---------

func (d *Do) exec(sql ...string) {
	var err error
	if len(sql) == 0 {
		d.sqlResult, err = d.db.Exec(d.sql, d.sqlVars...)
	} else {
		d.sqlResult, err = d.db.Exec(sql[0])
	}
	d.err(err)
}

func (d *Do) prepareDeleteSql() {
	d.sql = fmt.Sprintf(
		"DELETE FROM %v %v",
		d.tableName(),
		d.combinedSql(),
	)
}

func (d *Do) save() {
	if d.model.primaryKeyZero() {
		d.create()
	} else {
		d.update()
	}
	return
}

func (d *Do) delete() {
	d.prepareDeleteSql()
	if d.hasError() {
		return
	}
	d.exec()
}

func (d *Do) prepareCreateSql() {
	var sqls, columns []string

	for key, value := range d.model.columnsAndValues("create") {
		columns = append(columns, key)
		sqls = append(sqls, d.addToVars(value))
	}

	d.sql = fmt.Sprintf(
		"INSERT INTO %v (%v) VALUES (%v)",
		d.tableName(),
		strings.Join(columns, ","),
		strings.Join(sqls, ","),
	)
	return
}

func (d *Do) create() {
	d.prepareCreateSql()
	if d.hasError() {
		return
	}
	d.exec()
	if d.hasError() {
		return
	}
	id, err := d.sqlResult.LastInsertId()
	d.err(err)
	if d.hasError() {
		return
	}
	result := reflect.ValueOf(d.value).Elem()
	result.FieldByName(d.model.primaryKey()).SetInt(id)
}

func (d *Do) prepareUpdateSql() {
	updateAttrs := d.model.columnsAndValues("update")

	var sqls []string
	for key, value := range updateAttrs {
		sqls = append(sqls, fmt.Sprintf("%v = %v", key, d.addToVars(value)))
	}

	d.sql = fmt.Sprintf(
		"UPDATE %v SET %v %v",
		d.tableName(),
		strings.Join(sqls, ","),
		d.combinedSql(),
	)
}

func (d *Do) update() {
	d.prepareUpdateSql()
	if d.hasError() {
		return
	}
	d.exec()
}

func (d *Do) prepareQuerySql() {
	d.sql = fmt.Sprintf(
		"SELECT %v FROM %v %v",
		d.selectSql(),
		d.tableName(),
		d.combinedSql(),
	)
}

func (d *Do) query(where ...any) {

	if len(where) > 0 {
		d.where(where[0], where[1:len(where)]...)
	}

	destOut := reflect.Indirect(reflect.ValueOf(d.value))
	var destType reflect.Type
	var isSlice bool
	if x := destOut.Kind(); x == reflect.Slice {
		destType = destOut.Type().Elem()
		isSlice = true
	}

	d.prepareQuerySql()
	if d.hasError() {
		return
	}

	rows, err := d.db.Query(d.sql, d.sqlVars...)
	if d.err(err) != nil {
		return
	}
	defer rows.Close()
	if rows.Err() != nil {
		d.err(rows.Err())
	}

	counts := 0
	for rows.Next() {
		counts += 1
		var dest reflect.Value
		if isSlice {
			dest = reflect.New(destType).Elem()
		} else {
			dest = reflect.ValueOf(d.value).Elem()
		}
		columns, _ := rows.Columns()
		var values []any
		for _, value := range columns {
			field := dest.FieldByName(snakeToUpperCamel(value))
			if field.IsValid() {
				values = append(
					values,
					field.Addr().Interface(),
				)
			}
		}
		err := rows.Scan(values...)
		d.err(err)

		if isSlice {
			destOut.Set(reflect.Append(destOut, dest))
		}
	}
	if counts == 0 && !isSlice {
		d.err(errors.New("Record not found!"))
	}
}

func (d *Do) where(queryString any, args ...any) {
	d.whereClause = append(
		d.whereClause,
		map[string]any{
			"query": queryString,
			"args":  args,
		},
	)
}

func (d *Do) hasError() bool {
	return len(d.Errors) > 0
}

func (d *Do) setModel(value any) {
	d.model = &Model{data: value}
	d.value = value
}

func (d *Do) addToVars(value any) string {
	d.sqlVars = append(d.sqlVars, value)
	// return fmt.Sprintf("$%d", len(d.sqlVars))
	return "?"
}

func (d *Do) whereSql() (sql string) {

	var primaryCondition string
	if !d.model.primaryKeyZero() {
		primaryCondition = d.primaryCondition(d.addToVars(d.model.primaryKeyValue()))
	}

	var andConditions, orConditions []string
	for _, clause := range d.whereClause {
		andConditions = append(andConditions, d.buildWhereCondition(clause))
	}

	andSql := strings.Join(andConditions, " AND ")
	orSql := strings.Join(orConditions, " OR ")
	combinedConditions := andSql
	if len(combinedConditions) > 0 {
		if len(orSql) > 0 {
			combinedConditions = combinedConditions + " OR " + orSql
		}
	} else {
		combinedConditions = orSql
	}

	if len(primaryCondition) > 0 {
		sql = "WHERE " + primaryCondition
		if len(combinedConditions) > 0 {
			sql = sql + " AND ( " + combinedConditions + " )"
		}
	} else if len(combinedConditions) > 0 {
		sql = sql + "WHERE " + combinedConditions
	}
	return
}

func (d *Do) primaryCondition(value any) string {
	return fmt.Sprintf("(%v = %v)", d.model.primaryKeyDb(), value)
}

func (d *Do) buildWhereCondition(clause map[string]any) (str string) {
	switch clause["query"].(type) {
	case string:
		value := clause["query"].(string)
		str = "( " + value + " )"
	case int, int32, int64:
		return d.primaryCondition(d.addToVars(clause["query"]))
	}

	args := clause["args"].([]any)
	for _, arg := range args {
		d.addToVars(arg)
	}
	return
}

func (d *Do) selectSql() string {
	return "*"
}

func (d *Do) limitSql() string {
	if len(d.limitStr) == 0 {
		return ""
	} else {
		return " LIMIT " + d.limitStr
	}
}

func (d *Do) orderSql() string {
	if len(d.orderStrs) == 0 {
		return ""
	} else {
		return " ORDER BY " + strings.Join(d.orderStrs, ",")
	}
}

func (d *Do) combinedSql() string {
	return d.whereSql() + d.orderSql() + d.limitSql()
}

func (d *Do) createTable() *Do {
	var sqls []string
	for _, field := range d.model.fields("null") {
		sqls = append(sqls, field.DbName+" "+field.SqlType)
	}
	d.sql = fmt.Sprintf(
		"CREATE TABLE %v (%v)",
		d.tableName(),
		strings.Join(sqls, ","),
	)
	return d
}

func (d *Do) tableName() string {
	name, err := d.model.tableName()
	d.err(err)
	return name
}

func (d *Do) err(err error) error {
	if err != nil {
		d.Errors = append(d.Errors, err)
		d.chain.err(err)
	}
	return err
}

//--------- Model ---------

func (m *Model) tableName() (str string, err error) {
	if m.data == nil {
		err = errors.New("Model haven't been set")
		return
	}

	t := reflect.TypeOf(m.data)
	for {
		c := false
		switch t.Kind() {
		case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
			t = t.Elem()
			c = true
		}
		if !c {
			break
		}
	}

	str = toSnake(t.Name())

	pluralMap := map[string]string{"ch": "ches", "ss": "sses", "sh": "shes", "day": "days", "y": "ies", "x": "xes", "s?": "s"}
	for key, value := range pluralMap {
		reg := regexp.MustCompile(key + "$")
		if reg.MatchString(str) {
			return reg.ReplaceAllString(str, value), err
		}
	}
	return
}

func (m *Model) primaryKey() string {
	return "Id"
}
func (m *Model) primaryKeyDb() string {
	return toSnake(m.primaryKey())
}
func (m *Model) primaryKeyZero() bool {
	return m.primaryKeyValue() <= 0
}
func (m *Model) primaryKeyValue() int64 {
	if m.data == nil {
		return -1
	}
	t := reflect.TypeOf(m.data).Elem()
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
		return 0
	default:
		result := reflect.ValueOf(m.data).Elem()
		value := result.FieldByName(m.primaryKey())
		if value.IsValid() {
			return value.Interface().(int64)
		} else {
			return 0
		}
	}
}
func (m *Model) fields(operation string) (fields []Field) {
	typ := reflect.TypeOf(m.data).Elem()

	for i := 0; i < typ.NumField(); i++ {
		p := typ.Field(i)
		if !p.Anonymous {
			var field Field
			field.Name = p.Name
			field.DbName = toSnake(p.Name)
			field.IsPrimaryKey = m.primaryKeyDb() == field.DbName
			field.AutoCreateTime = "created_at" == field.DbName
			field.AutoUpdateTime = "updated_at" == field.DbName
			value := reflect.ValueOf(m.data).Elem().FieldByName(p.Name)

			switch operation {
			case "create":
				if (field.AutoCreateTime || field.AutoUpdateTime) &&
					value.Interface().(time.Time).IsZero() {
					value.Set(reflect.ValueOf(time.Now()))
				}
			case "update":
				if field.AutoUpdateTime {
					value.Set(reflect.ValueOf(time.Now()))
				}
			}

			field.Value = value.Interface()

			if field.IsPrimaryKey {
				field.SqlType = getPrimaryKeySqlType(field.Value, 0)
			} else {
				field.SqlType = getSqlType(field.Value, 0)
			}
			fields = append(fields, field)
		}
	}
	return
}

func (m *Model) columnsAndValues(operation string) map[string]any {
	results := map[string]any{}
	for _, field := range m.fields(operation) {
		if !field.IsPrimaryKey {
			results[field.DbName] = field.Value
		}
	}
	return results
}

//--------- SqlType ---------

func getPrimaryKeySqlType(column interface{}, size int) string {
	suffix_str := " NOT NULL AUTO_INCREMENT PRIMARY KEY"
	switch column.(type) {
	case int, int8, int16, int32, uint, uint8, uint16, uint32:
		return "int" + suffix_str
	case int64, uint64:
		return "bigint" + suffix_str
	}
	panic("unsupported sql adaptor, please submit an issue in github")
}

func getSqlType(column interface{}, size int) string {
	switch column.(type) {
	case time.Time:
		return "timestamp"
	case bool:
		return "boolean"
	case int, int8, int16, int32, uint, uint8, uint16, uint32:
		return "int"
	case int64, uint64:
		return "bigint"
	case float32, float64:
		return "double"
	case []byte:
		if size > 0 && size < 65532 {
			return fmt.Sprintf("varbinary(%d)", size)
		}
		return "longblob"
	case string:
		if size > 0 && size < 65532 {
			return fmt.Sprintf("varchar(%d)", size)
		}
		return "longtext"
	default:
		panic("invalid sql type")
	}
}

//--------- utils ---------

func toSnake(s string) string {
	buf := bytes.NewBufferString("")
	for i, v := range s {
		if i > 0 && v >= 'A' && v <= 'Z' {
			buf.WriteRune('_')
		}
		buf.WriteRune(v)
	}
	return strings.ToLower(buf.String())
}

func snakeToUpperCamel(s string) string {
	buf := bytes.NewBufferString("")
	for _, v := range strings.Split(s, "_") {
		if len(v) > 0 {
			buf.WriteString(strings.ToUpper(v[:1]))
			buf.WriteString(v[1:])
		}
	}
	return buf.String()
}
