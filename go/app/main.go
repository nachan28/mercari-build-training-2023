package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	err = json.Unmarshal(data, items)
	return items, nil
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
	var itemWrapper ItemWrapper
	itemWrapper, err := readItemsFromFile()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read json file"})
	}
	

	// Add item to itemWrapper
	itemWrapper.Items = append(itemWrapper.Items, item)

	// Clear and rewrite items into items.json
	itemsJsonData, err := json.Marshal(itemWrapper)
	if err != nil {
		c.Logger().Error(err)
	}
	os.WriteFile("items.json", itemsJsonData, 0666)
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(itemWrapper); err != nil {
		log.Fatal(err)
	}
	
	// Return message
	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}
	return c.JSON(http.StatusOK, res)
}

func getAllItems(c echo.Context) error {
	// Read items.json
	allItems, err := readItemsFromFile()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read json file"})
	}

	// Return response
	message := string(allItems)
	res := Response{Message: message}
	return c.JSON(http.StatusOK, res)
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
	e.GET("/image/:imageFilename", getImg)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}