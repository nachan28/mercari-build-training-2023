package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

const (
	ImgDir = "images"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
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
		return ItemWrapper{}, err
	}
	var items ItemWrapper
	err = json.Unmarshal(data, &items)
	if err != nil {
		return ItemWrapper{}, err
	}
	return items, nil
}

func writeItemsToJSON(items ItemWrapper) error{
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
		} else{
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
	
	// Create item object
	item := Item{name, category, hashImageName + ".jpg"}
	c.Logger().Infof("Receive item: %s, category: %s", name, category)



	// Read data from items.json
	itemWrapper, err := readItemsFromFile()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read json file"})
	}
	
	// Add item to itemWrapper
	itemWrapper.Items = append(itemWrapper.Items, item)

	// Write data to items.json
	err = writeItemsToJSON(itemWrapper)
	if err != nil {
		log.Fatal(err)
	}

	// Return message
	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}
	return c.JSON(http.StatusOK, res)
}

func getAllItems(c echo.Context) error {
	// Read items.json
	items, err := readItemsFromFile()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read json file"})
	}
	return c.JSON(http.StatusOK, items)
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
	e.Logger.SetLevel(log.INFO)

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