package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ImgDir = "images"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	Id           int
	Name         string `json:"name"`
	Category     string `json:"category"`
	Img_filename string `json:"img_filename"`
}

type ItemWrapper struct {
	Items []Item `json:"items"`
}

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func trimPath(s string) string {
	imgFileName := filepath.Base(s)
	img := strings.TrimSuffix(imgFileName, filepath.Ext(imgFileName))
	return img
}

func readItemsFromFile() (ItemWrapper, error) {
	data, err := os.ReadFile("items.json")
	if err != nil {
		log.Printf("Failed to unmarshal items.json: %v", err)
		return ItemWrapper{}, err
	}

	var items ItemWrapper

	if len(data) == 0 {
		err = writeItemsToJSON(ItemWrapper{})
		if err != nil {
			log.Printf("Failed to write to items.json: %v", err)
			return ItemWrapper{}, err
		}
	} else {
		err = json.Unmarshal(data, &items)
		if err != nil {
			log.Printf("Failed to read items.json: %v", err)
			return ItemWrapper{}, err
		}
	}
	return items, nil
}

func writeItemsToJSON(items ItemWrapper) error {
	itemsJsonData, err := json.Marshal(items)
	if err != nil {
		return err
	}
	err = os.WriteFile("items.json", itemsJsonData, 0666)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, err := os.Create("items.json")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func addItem(c echo.Context) error {
	// Get form data
	name := c.FormValue("name")
	category := c.FormValue("category")
	imagePath := c.FormValue("image")

	// Hash image
	img := trimPath(imagePath)
	hashImageName := hashString(img)

	// Connect to DB
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Insert item to items table
	cmd := "INSERT INTO items (name, category, image_filename) VALUES($1, $2, $3)"
	_, err = db.Exec(cmd, name, category, hashImageName+".jpg")
	if err != nil {
		log.Fatal(err)
	}
	// Return message
	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}
	return c.JSON(http.StatusOK, res)
}

func getAllItems(c echo.Context) error {
	// Connect to DB
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get all records from items table
	cmd := "SELECT * FROM items"
	rows, err := db.Query(cmd)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var items ItemWrapper

	// Return response
	for rows.Next() {
		var item Item

		err := rows.Scan(&item.Id, &item.Name, &item.Category, &item.Img_filename)
		if err != nil {
			log.Fatal(err)
		}

		items.Items = append(items.Items, item)
	}
	return c.JSON(http.StatusOK, items)
}

func getItem(c echo.Context) error {
	// Get param
	idParam := c.Param("item_id")
	itemId, err := strconv.Atoi(idParam)
	if err != nil {
		return err
	}

	// Connect to DB
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get target record from items table
	cmd := "SELECT * FROM items WHERE id=$1"
	rows, err := db.Query(cmd, itemId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Return response
	if rows.Next() {
		var item Item
		err = rows.Scan(&item.Id, &item.Name, &item.Category, &item.Img_filename)
		if err != nil {
			log.Fatal(err)
		}
		return c.JSON(http.StatusOK, item)
	} else {
		res := Response{Message: "Not found"}
		return c.JSON(http.StatusNotFound, res)
	}
}

func getImg(c echo.Context) error {
	// Create image path
	imgPath := path.Join(ImgDir, c.Param("imageFilename"))

	if !strings.HasSuffix(imgPath, ".jpg") {
		res := Response{Message: "Image path does not end with .jpg"}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, err := os.Stat(imgPath); err != nil {
		c.Logger().Debugf("Image not found: %s", imgPath)
		imgPath = path.Join(ImgDir, "default.jpg")
	}
	return c.File(imgPath)
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.DEBUG)

	front_url := os.Getenv("FRONT_URL")
	if front_url == "" {
		front_url = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{front_url},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Routes
	e.GET("/", root)
	e.POST("/items", addItem)
	e.GET("/items", getAllItems)
	e.GET("/items/:item_id", getItem)
	e.GET("/image/:imageFilename", getImg)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}
