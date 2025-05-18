# gormysql

## Usage

```go
import "github.com/demouth/gormysql"

// For MySQL
// db, err := gormysql.Open("user:password@tcp(localhost:3306)/dbname")

// Specify any driver
// db, err := gormysql.OpenWithDriver("mysql", "user:password@tcp(localhost:3306)/dbname")

// Query examples
// db.Exec("CREATE TABLE ...")
// db.Where("id = ?", 1).Find(&result)
```