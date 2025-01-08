package gormysql_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/demouth/gormysql"
)

type User struct {
	Id        int64
	Age       int64
	Birthday  time.Time
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Product struct {
	Id                    int64
	Code                  string
	Price                 int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	BeforeCreateCallTimes int64
	AfterCreateCallTimes  int64
	BeforeUpdateCallTimes int64
	AfterUpdateCallTimes  int64
	BeforeSaveCallTimes   int64
	AfterSaveCallTimes    int64
	BeforeDeleteCallTimes int64
	AfterDeleteCallTimes  int64
}

var (
	db                 gormysql.DB
	t1, t2, t3, t4, t5 time.Time
)

func init() {
	var err error
	db, err = gormysql.Open("gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		panic(fmt.Sprintf("No error should happen when connect database, but got %+v", err))
	}

	err = db.Exec("drop table IF EXISTS users;").Error
	if err != nil {
		fmt.Printf("Got error when try to delete table uses, %+v\n", err)
	}

	db.Exec("drop table IF EXISTS products;")

	orm := db.CreateTable(&User{})
	if orm.Error != nil {
		panic(fmt.Sprintf("No error should happen when create table, but got %+v", orm.Error))
	}

	db.CreateTable(&Product{})

	var shortForm = "2006-01-02 15:04:05"
	t1, _ = time.Parse(shortForm, "2000-10-27 12:02:40")
	t2, _ = time.Parse(shortForm, "2002-01-01 00:00:00")
	t3, _ = time.Parse(shortForm, "2005-01-01 00:00:00")
	t4, _ = time.Parse(shortForm, "2010-01-01 00:00:00")
	t5, _ = time.Parse(shortForm, "2020-01-01 00:00:00")
	orm = db.Save(&User{Name: "1", Age: 18, Birthday: t1})
	if orm.Error != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", orm.Error))
	}
	db.Save(&User{Name: "2", Age: 20, Birthday: t2})
	db.Save(&User{Name: "3", Age: 22, Birthday: t3})
	db.Save(&User{Name: "3", Age: 24, Birthday: t4})
	db.Save(&User{Name: "5", Age: 26, Birthday: t4})
}

func TestInitlineCondition(t *testing.T) {
	var u1, u2, u3, u4, u5, u6, u7 User
	db.Where("name = ?", "3").Order("age desc").First(&u1).First(&u2)

	db.Where("name = ?", "3").First(&u3, "age = 22").First(&u4, "age = ?", 24).First(&u5, "age = ?", 26)
	if !((u5.Id == 0) && (u3.Age == 22 && u3.Name == "3") && (u4.Age == 24 && u4.Name == "3")) {
		t.Errorf("Inline where condition for first when search")
	}

	var us1, us2, us3, us4 []User
	db.Find(&us1, "age = 22").Find(&us2, "name = ?", "3").Find(&us3, "age > ?", 20)
	if !(len(us1) == 1 && len(us2) == 2 && len(us3) == 3) {
		t.Errorf("Inline where condition for find when search")
	}

	db.Find(&us4, "name = ? and age > ?", "3", "22")
	if len(us4) != 1 {
		t.Errorf("More complex inline where condition for find, %v", us4)
	}

	db.First(&u6, u1.Id)
	if !(u6.Id == u1.Id && u6.Id != 0) {
		t.Errorf("Should find out user with int id")
	}

	db.First(&u7, strconv.Itoa(int(u1.Id)))
	if !(u6.Id == u1.Id && u6.Id != 0) {
		t.Errorf("Should find out user with string id")
	}
}

func TestSaveAndFind(t *testing.T) {
	name := "save_and_find"
	u := &User{Name: name, Age: 1, Birthday: t1}
	orm := db.Save(u)
	if orm.Error != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", orm.Error))
	}
	if u.Id == 0 {
		t.Errorf("Should have ID after create record")
	}

	user := &User{}
	db.First(user, "name = ?", name)
	if user.Name != name {
		t.Errorf("User should be saved and fetched correctly")
	}

	users := []User{}
	db.Find(&users)
}

func TestSaveAndUpdate(t *testing.T) {
	name, name2, new_name := "update", "update2", "new_update"
	user := User{Name: name, Age: 1, Birthday: t1}
	var err error
	err = db.Save(&user).Error
	if err != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", err))
	}
	err = db.Save(&User{Name: name2, Age: 1, Birthday: t1}).Error
	if err != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", err))
	}

	if user.Id == 0 {
		t.Errorf("User Id should exist after create")
	}

	user.Name = new_name
	db.Save(&user)
	orm := db.Where("name = ?", name).First(&User{})
	if orm.Error == nil {
		t.Errorf("Should raise error when looking for a existing user with an outdated name")
	}

	orm = db.Where("name = ?", new_name).First(&User{})
	if orm.Error != nil {
		t.Errorf("Shouldn't raise error when looking for a existing user with the new name")
	}

	orm = db.Where("name = ?", name2).First(&User{})
	if orm.Error != nil {
		t.Errorf("Shouldn't update other users")
	}
}

func TestDelete(t *testing.T) {
	name, name2 := "delete", "delete2"
	user := User{Name: name, Age: 1, Birthday: t1}
	var err error
	err = db.Save(&user).Error
	if err != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", err))
	}
	err = db.Save(&User{Name: name2, Age: 1, Birthday: t1}).Error
	if err != nil {
		panic(fmt.Sprintf("No error should happen when save, but got %+v", err))
	}
	orm := db.Delete(&user)

	orm = db.Where("name = ?", name).First(&User{})
	if orm.Error == nil {
		t.Errorf("User should be deleted successfully")
	}

	orm = db.Where("name = ?", name2).First(&User{})
	if orm.Error != nil {
		t.Errorf("User2 should not be deleted, but got %+v", orm.Error)
	}
}

func TestWhere(t *testing.T) {
	name := "where"
	db.Save(&User{Name: name, Age: 1, Birthday: t1})

	user := &User{}
	db.Where("name = ?", name).First(user)
	if user.Name != name {
		t.Errorf("Should found out user with name '%v'", name)
	}

	if db.Where(user.Id).First(&User{}).Error != nil {
		t.Errorf("Should found out users only with id")
	}

	user = &User{}
	orm := db.Where("name LIKE ?", "%nonono%").First(user)
	if orm.Error == nil {
		t.Errorf("Should return error when searching for none existing record, %+v", user)
	}

	user = &User{}
	orm = db.Where("name LIKE ?", "%whe%").First(user)
	if orm.Error != nil {
		t.Errorf("Should not return error when searching for existing record, %+v", user)
	}

	user = &User{}
	orm = db.Where("name = ?", "noexisting-user").First(user)
	if orm.Error == nil {
		t.Errorf("Should return error when looking for none existing record, %+v", user)
	}

	users := []User{}
	orm = db.Where("name = ?", "none-noexisting").Find(&users)
	if orm.Error != nil {
		t.Errorf("Shouldn't return error when looking for none existing records, %+v", users)
	}
	if len(users) != 0 {
		t.Errorf("Shouldn't find anything when looking for none existing records, %+v", users)
	}
}

func TestComplexWhere(t *testing.T) {
	var users []User
	db.Where("age > ?", 20).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should only found 3 users that age > 20, but have %v", len(users))
	}

	users = []User{}
	db.Where("age >= ?", 20).Find(&users)
	if len(users) != 4 {
		t.Errorf("Should only found 4 users that age >= 20, but have %v", len(users))
	}

	users = []User{}
	db.Where("age = ?", 20).Find(&users)
	if len(users) != 1 {
		t.Errorf("Should only found 1 users age == 20, but have %v", len(users))
	}

	users = []User{}
	db.Where("age <> ?", 20).Find(&users)
	if len(users) < 3 {
		t.Errorf("Should have more than 3 users age != 20, but have %v", len(users))
	}

	users = []User{}
	db.Where("name = ? and age >= ?", "3", 20).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should only found 2 users that age >= 20 with name 3, but have %v", len(users))
	}

	users = []User{}
	db.Where("name = ?", "3").Where("age >= ?", 20).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should only found 2 users that age >= 20 with name 3, but have %v", len(users))
	}

	users = []User{}
	db.Where("birthday > ?", t2).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should only found 3 users's birthday >= t2, but have %v", len(users))
	}

	users = []User{}
	db.Where("birthday >= ?", t1).Where("birthday < ?", t2).Find(&users)
	if len(users) != 1 {
		t.Errorf("Should only found 1 users's birthday <= t2, but have %v", len(users))
	}

	users = []User{}
	db.Where("birthday >= ? and birthday <= ?", t1, t2).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should only found 2 users's birthday <= t2, but have %v", len(users))
	}

	users = []User{}
	db.Where("name in (?)", []string{"1", "3"}).Find(&users)

	if len(users) != 3 {
		t.Errorf("Should only found 3 users's name in (1, 3), but have %v", len(users))
	}

	var user_ids []int64
	for _, user := range users {
		user_ids = append(user_ids, user.Id)
	}
	users = []User{}
	db.Where("id in (?)", user_ids).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should only found 3 users's name in (1, 3) - search by id, but have %v", len(users))
	}

	users = []User{}
	db.Where("name in (?)", []string{"1", "2"}).Find(&users)

	if len(users) != 2 {
		t.Errorf("Should only found 2 users's name in (1, 2), but have %v", len(users))
	}

	user_ids = []int64{}
	for _, user := range users {
		user_ids = append(user_ids, user.Id)
	}
	users = []User{}
	db.Where("id in (?)", user_ids).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should only found 2 users's name in (1, 2) - search by id, but have %v", len(users))
	}

	users = []User{}
	db.Where("id in (?)", user_ids[0]).Find(&users)
	if len(users) != 1 {
		t.Errorf("Should only found 1 users's name in (1, 2) - search by the first id, but have %v", len(users))
	}
}
