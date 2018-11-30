package bankapi

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type Server struct {
	db          *sql.DB
	userService UserService
}

type UserService interface {
	All() ([]User, error)
	Insert(user *User) error
	GetByID(id int) (*User, error)
	//DeleteByID(id int) error
	//Update(id int, body string) (*User, error)
}

type UserServiceImp struct {
	db *sql.DB
}

var ErrNotFound = errors.New("user: not found")

type User struct {
	mu        sync.Mutex
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name" binding:"required"`
	LastName  string    `json:"last_name" binding:"required"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type BankAccount struct {
	mu            sync.Mutex
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	AccountNumber string    `json:"account_number"`
	Name          string    `json:"Name"`
	Balance       int       `json:"balance"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Secret struct {
	ID  int64  `json:"id"`
	Key string `json:"key" binding:"required"`
}

func (s *UserServiceImp) All() ([]User, error) {
	rows, err := s.db.Query("SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	users := []User{} // set empty slice without nil
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.UpdatedAt, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *UserServiceImp) Insert(user *User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	row := s.db.QueryRow("INSERT INTO users (first_name, last_name, created_at, updated_at) values ($1, $2, $3, $4) RETURNING id", user.FirstName, user.LastName, now, now)

	if err := row.Scan(&user.ID); err != nil {
		return err
	}
	return nil
}

func (s *UserServiceImp) GetByID(id int) (*User, error) {
	stmt := "SELECT * FROM todos WHERE id = $1"
	row := s.db.QueryRow(stmt, id)
	var todo User
	err := row.Scan(&todo.ID, &todo.FirstName, &todo.LastName, &todo.CreatedAt, &todo.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &todo, nil
}

func AccessLogWrap(hand http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path)
		hand.ServeHTTP(w, r)
	})
}

func (s *Server) All(c *gin.Context) {
	todos, err := s.userService.All()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"object":  "error",
			"message": fmt.Sprintf("db: query error: %s", err),
		})
		return
	}
	c.JSON(http.StatusOK, todos)
}

func (s *Server) Create(c *gin.Context) {
	var user User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"object":  "error",
			"message": fmt.Sprintf("json: wrong params: %s", err),
		})
		return
	}

	if err := s.userService.Insert(&user); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (s *Server) GetByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	user, err := s.userService.GetByID(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func setupRoute(s *Server) *gin.Engine {
	r := gin.Default()
	users := r.Group("/users")
	//admin := r.Group("/admin")

	//admin.Use(gin.BasicAuth(gin.Accounts{
	//	"admin": "1234",
	//}))
	//users.Use(s.AuthTodo)
	users.GET("/", s.All)
	users.POST("/", s.Create)
	//
	//todos.GET("/:id", s.GetByID)
	//todos.PUT("/:id", s.Update)
	//todos.DELETE("/:id", s.DeleteByID)
	//admin.POST("/secrets", s.CreateSecret)
	return r
}

func StartServer() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return
	}
	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		first_name TEXT,
		last_name TEXT,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
	);
	CREATE TABLE IF NOT EXISTS bank_accounts (
		id SERIAL PRIMARY KEY,
		user_id SERIAL,
		account_number TEXT UNIQUE,
		account_name TEXT,
		balance TEXT,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
	);
	`

	if _, err := db.Exec(createTable); err != nil {
		fmt.Printf("%s", err)
		return
	}

	s := &Server{
		db: db,
		userService: &UserServiceImp{
			db: db,
		},
	}

	r := setupRoute(s)

	r.Run(":" + os.Getenv("PORT"))
}
