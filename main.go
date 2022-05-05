package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"

	"github.com/rs/cors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type address struct {
	Id       int     `json:"id"`
	Long     float64 `json:"long"`
	Lat      float64 `json:"lat"`
	Location string  `json:"location"`
}

type allAddress []address
type Geo struct {
	Id       int
	Location string
	Lat      float64
	Long     float64
}

type GeoPoint struct {
	Geo                   *Geo
	PreviousStep          *GeoPoint
	DistanceFromLastPoint float64
	TotalCoveredDistance  float64
}

type Route struct {
	Steps []GeoPoint
}
type CallbackEnd func()

func main() {

	router := mux.NewRouter().StrictSlash(true)
	/* 	router.HandleFunc("/", getOneEvent).Methods("GET")
	   	router.HandleFunc("/address", homeLink).Methods("GET")
	   	router.HandleFunc("/createRoute/{id}", createRouter).Methods("POST") */
	router.HandleFunc("/", getOneEvent).Methods("GET")
	router.HandleFunc("/createRoute/{id}", createRouter).Methods("POST")
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080", "https://melodious-bublanina-19bee5.netlify.app"},
		AllowCredentials: true,
	})

	handler := c.Handler(router)
	port := os.Getenv(("PORT"))
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
func Factorial(x int) int {
	if x == 0 {
		return 1
	}

	return x * Factorial(x-1)
}

// returns the mathematical distance between two geo points (lat + long)
// the returned value is either "m" for meter or "" for a raw result.
// this func aims to compare several distance from different points
func Distance(lat1 float64, lng1 float64, lat2 float64, lng2 float64, unit string) float64 {
	const PI float64 = 3.141592653589793

	radlat1 := float64(PI * lat1 / 180)
	radlat2 := float64(PI * lat2 / 180)

	theta := float64(lng1 - lng2)
	radtheta := float64(PI * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)
	if dist > 1 {
		dist = 1
	}
	dist = math.Acos(dist)
	if unit == "m" {
		dist = dist * 180 / PI
		dist = dist * 60 * 1.1515

		dist = dist * 1.609344 * 1000 //m
	}

	return dist
}

// returns the mathematical distance between two geo points (lat + long)
// this func aims to compare several distance from different points
func RawDistance(lat1 float64, lng1 float64, lat2 float64, lng2 float64) float64 {
	return Distance(lat1, lng1, lat2, lng2, "")
}
func initSQL(uri string) *sql.DB {
	db, err := sql.Open(`mysql`, uri)
	if err != nil {
		fmt.Println("error")
	}

	return db
}
func getSQLData(sqlR *sql.DB) allAddress {
	var addressSelected allAddress

	results, err := sqlR.Query("select * from geopoint")
	if err != nil {
		fmt.Println("error")
	}
	for results.Next() {
		var addressFinal address
		err = results.Scan(&addressFinal.Id, &addressFinal.Lat, &addressFinal.Long, &addressFinal.Location)
		if err != nil {
			fmt.Println("error")
		}
		//fmt.Printf("lat: %f, long: %f", addressFinal.Lat, addressFinal.Long)

		//fmt.Printf("\n")
		addressSelected = append(addressSelected, addressFinal)
	}
	defer sqlR.Close()
	return addressSelected
}
func getOneEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	sqlURI := os.Getenv(("DATABASE_URL"))
	sqlR := initSQL(sqlURI)
	allAddress := getSQLData(sqlR)
	json.NewEncoder(w).Encode(allAddress)
}
func createRouter(w http.ResponseWriter, r *http.Request) {
	fmt.Println("entre")
	var addressData []Geo
	var finalRoute Route
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		fmt.Println("id is missing in parameters")
	}
	fmt.Println(`id := `, id)
	fmt.Println(`r.Body := `, r.Body)
	json.NewDecoder(r.Body).Decode(&addressData)
	for _, singleAddress := range addressData {
		fmt.Println("singleAddress:", singleAddress.Location)
	}
	finalRoute = GetBestRoute(addressData, 1000000)
	for _, singleAddress := range finalRoute.Steps {
		fmt.Println("Step:", singleAddress.Geo.Location)
	}
	w.WriteHeader(http.StatusOK)
	jsonResponse, jsonError := json.Marshal(finalRoute)

	if jsonError != nil {
		fmt.Println("Unable to encode JSON")
	}
	w.Write(jsonResponse)
}
func homeLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, "Welcome home!")
}
func duplicateWithoutOneSale(geopoints []Geo, index int) []Geo {
	salesLen := len(geopoints)
	duplicatedSales := make([]Geo, salesLen)
	copy(duplicatedSales, geopoints)
	duplicatedSales[index] = duplicatedSales[salesLen-1] //replace geo to delete with the last geo
	return duplicatedSales[:salesLen-1]                  //return a duplicated array without the last element
}

func createStep(lastStep GeoPoint, geo Geo) GeoPoint {
	distanceFromLastStep := 0.0
	totalCoveredDistance := 0.0
	if lastStep.Geo != nil {
		distanceFromLastStep = RawDistance(lastStep.Geo.Lat, lastStep.Geo.Long, geo.Lat, geo.Long)
		totalCoveredDistance = lastStep.TotalCoveredDistance + distanceFromLastStep
	}

	step := GeoPoint{&geo, &lastStep, distanceFromLastStep, totalCoveredDistance}
	return step
}

//provide to channel every possible values
func createRoutes(routesChan chan<- GeoPoint, geopoints []Geo, lastStep GeoPoint, safeGuard chan int, maxGoroutines int, onEnd CallbackEnd) {
	if len(geopoints) == 1 {
		routesChan <- createStep(lastStep, geopoints[0])
	} else {
		for index, geo := range geopoints {
			// transform the geo as a step
			step := createStep(lastStep, geo)

			// remove the geo from the current list of geopoints
			remainingSales := duplicateWithoutOneSale(geopoints, index)

			if len(safeGuard) < maxGoroutines {
				safeGuard <- 1
				releaseSafeGuard := func() { <-safeGuard }
				go createRoutes(routesChan, remainingSales, step, safeGuard, maxGoroutines, releaseSafeGuard)
			} else {
				createRoutes(routesChan, remainingSales, step, safeGuard, maxGoroutines, nil)
			}
		}
	}

	if onEnd != nil {
		onEnd()
	}
}

// Compute the best route to link a number of geopoints, each geo being represented by geo coordinates.
func GetBestRoute(geopoints []Geo, maxGoroutines int) Route {

	safeGuard := make(chan int, maxGoroutines)
	routesChan := make(chan GeoPoint)

	// Create routes
	go createRoutes(routesChan, geopoints, GeoPoint{}, safeGuard, maxGoroutines, nil)

	// Find best route
	bestStep := GeoPoint{nil, nil, 0, 0}
	numberOfSolutions := Factorial(len(geopoints))
	stepIndex := 0
	for step := range routesChan {

		if bestStep.Geo == nil || step.TotalCoveredDistance < bestStep.TotalCoveredDistance {
			bestStep = step
		}
		stepIndex++
		if stepIndex == numberOfSolutions {
			close(routesChan)
		}
	}

	//format route as an array of step
	steps := []GeoPoint{}
	for step := bestStep; step.Geo != nil; step = *step.PreviousStep {
		steps = append([]GeoPoint{step}, steps...)
	}
	return Route{steps}
}
