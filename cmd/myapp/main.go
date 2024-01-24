package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

var (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "password"
	dbname   = "tambola"
	db       *sql.DB
)

type IntArray []int

func (a *IntArray) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:

		str := string(src)
		str = strings.Trim(str, "{}")
		parts := strings.Split(str, ",")

		for _, part := range parts {
			val, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return err
			}
			*a = append(*a, val)
		}

		return nil
	default:
		return fmt.Errorf("pq: unable to convert %T to IntArray", src)
	}
}

func init() {

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id SERIAL PRIMARY KEY,
			numbers INT ARRAY
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to the database")
}

func generateTicket() ([]int, error) {

	ticket := make([][]int, 3)
	for i := range ticket {
		ticket[i] = make([]int, 9)
	}

	for row := 0; row < 3; row++ {
		usedColumns := make(map[int]bool)
		for count := 0; count < 5; count++ {
			column := rand.Intn(9)
			for usedColumns[column] {
				column = rand.Intn(9)
			}
			usedColumns[column] = true

			ticket[row][column] = rand.Intn(10) + (column * 10) + 1
		}
	}


	var flattenedTicket []int
	for _, row := range ticket {
		flattenedTicket = append(flattenedTicket, row...)
	}

	return flattenedTicket, nil
}


func insertTicket(ticket []int) error {

	_, err := db.Exec("INSERT INTO tickets (numbers) VALUES ($1)", pq.Array(ticket))
	return err
}
type ticketResponse struct {
	Tickets map[int][]int `json:"tickets"`
}

func main() {
	router := gin.Default()

	router.POST("/generate/:sets", func(c *gin.Context) {
		sets, err := strconv.Atoi(c.Param("sets"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sets parameter"})
			return
		}

		for i := 0; i < sets; i++ {
			ticket, err := generateTicket()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating ticket"})
				return
			}

			err = insertTicket(ticket)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error inserting ticket into the database"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Tickets generated and saved successfully"})
	})

	router.GET("/tickets", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

		offset := (page - 1) * pageSize
		rows, err := db.Query("SELECT id, numbers FROM tickets ORDER BY id LIMIT $1 OFFSET $2", pageSize, offset)
		if err != nil {
			log.Printf("Error querying tickets: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching tickets from the database"})
			return
		}
		defer rows.Close()

		var tickets map[int][]int
		tickets = make(map[int][]int)

		for rows.Next() {
			var id int
			var numbers pq.Int64Array
			err := rows.Scan(&id, &numbers)
			if err != nil {
				log.Printf("Error scanning row: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning rows"})
				return
			}


			intArray := make([]int, len(numbers))
			for i, num := range numbers {
				intArray[i] = int(num)
			}

			tickets[id] = intArray
		}

		c.JSON(http.StatusOK, ticketResponse{Tickets: tickets})
	})

	router.Run(":8080")

	defer db.Close()
}
