// Package testutil provides test utilities including mock Hue bridge
package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockBridge simulates a Philips Hue bridge for testing
type MockBridge struct {
	Server *httptest.Server
	mu     sync.RWMutex

	// State
	Scenes         map[string]*MockScene
	Lights         map[string]*MockLight
	GroupedLights  map[string]*MockGroupedLight
	Entertainment  map[string]*MockEntertainment
	Rooms          map[string]*MockRoom
	Zones          map[string]*MockZone
	RequestLog     []string
	ResponseErrors map[string]error // URL path -> error to return
}

type MockScene struct {
	ID       string
	Name     string
	RoomName string
	Type     string
}

type MockLight struct {
	ID         string
	Name       string
	On         bool
	Brightness float32
	Color      struct {
		XY struct {
			X float32 `json:"x"`
			Y float32 `json:"y"`
		} `json:"xy"`
	}
}

type MockGroupedLight struct {
	ID         string
	Name       string
	Type       string
	On         bool
	Brightness float32
}

type MockEntertainment struct {
	ID     string
	Name   string
	Type   string
	Status string
}

type MockRoom struct {
	ID   string
	Name string
}

type MockZone struct {
	ID   string
	Name string
}

// NewMockBridge creates a new mock Hue bridge server
func NewMockBridge() *MockBridge {
	mb := &MockBridge{
		Scenes:         make(map[string]*MockScene),
		Lights:         make(map[string]*MockLight),
		GroupedLights:  make(map[string]*MockGroupedLight),
		Entertainment:  make(map[string]*MockEntertainment),
		Rooms:          make(map[string]*MockRoom),
		Zones:          make(map[string]*MockZone),
		RequestLog:     []string{},
		ResponseErrors: make(map[string]error),
	}

	mb.Server = httptest.NewServer(http.HandlerFunc(mb.handleRequest))
	return mb
}

// NewMockBridgeTLS creates a mock bridge with TLS support
func NewMockBridgeTLS() *MockBridge {
	mb := &MockBridge{
		Scenes:         make(map[string]*MockScene),
		Lights:         make(map[string]*MockLight),
		GroupedLights:  make(map[string]*MockGroupedLight),
		Entertainment:  make(map[string]*MockEntertainment),
		Rooms:          make(map[string]*MockRoom),
		Zones:          make(map[string]*MockZone),
		RequestLog:     []string{},
		ResponseErrors: make(map[string]error),
	}

	mb.Server = httptest.NewTLSServer(http.HandlerFunc(mb.handleRequest))
	return mb
}

// Close shuts down the mock server
func (mb *MockBridge) Close() {
	mb.Server.Close()
}

// URL returns the mock bridge URL
func (mb *MockBridge) URL() string {
	return mb.Server.URL
}

// AddScene adds a mock scene
func (mb *MockBridge) AddScene(id, name, roomName string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.Scenes[id] = &MockScene{
		ID:       id,
		Name:     name,
		RoomName: roomName,
		Type:     "scene",
	}
}

// AddGroupedLight adds a mock grouped light (room/zone)
func (mb *MockBridge) AddGroupedLight(id, name, typ string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.GroupedLights[id] = &MockGroupedLight{
		ID:         id,
		Name:       name,
		Type:       typ,
		On:         true,
		Brightness: 100.0,
	}
}

// AddLight adds a mock light
func (mb *MockBridge) AddLight(id, name string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.Lights[id] = &MockLight{
		ID:         id,
		Name:       name,
		On:         true,
		Brightness: 100.0,
	}
}

// AddRoom adds a mock room
func (mb *MockBridge) AddRoom(id, name string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.Rooms[id] = &MockRoom{
		ID:   id,
		Name: name,
	}
}

// AddZone adds a mock zone
func (mb *MockBridge) AddZone(id, name string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.Zones[id] = &MockZone{
		ID:   id,
		Name: name,
	}
}

// SetResponseError sets an error to return for a specific URL path
func (mb *MockBridge) SetResponseError(path string, err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.ResponseErrors[path] = err
}

// ClearResponseErrors clears all response errors
func (mb *MockBridge) ClearResponseErrors() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.ResponseErrors = make(map[string]error)
}

// GetRequestCount returns the number of requests received
func (mb *MockBridge) GetRequestCount() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return len(mb.RequestLog)
}

// GetRequestLog returns a copy of the request log
func (mb *MockBridge) GetRequestLog() []string {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	logCopy := make([]string, len(mb.RequestLog))
	copy(logCopy, mb.RequestLog)
	return logCopy
}

// ClearRequestLog clears the request log
func (mb *MockBridge) ClearRequestLog() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.RequestLog = []string{}
}

// handleRequest handles HTTP requests to the mock bridge
func (mb *MockBridge) handleRequest(w http.ResponseWriter, r *http.Request) {
	mb.mu.Lock()
	mb.RequestLog = append(mb.RequestLog, fmt.Sprintf("%s %s", r.Method, r.URL.Path))

	// Check for error override
	if err, ok := mb.ResponseErrors[r.URL.Path]; ok {
		mb.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mb.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	// Route requests
	switch {
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/scene":
		mb.handleGetScenes(w, r)
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/grouped_light":
		mb.handleGetGroupedLights(w, r)
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/light":
		mb.handleGetLights(w, r)
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/room":
		mb.handleGetRooms(w, r)
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/zone":
		mb.handleGetZones(w, r)
	case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/entertainment_configuration":
		mb.handleGetEntertainment(w, r)
	case r.Method == "PUT":
		mb.handlePutRequest(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleGetScenes returns mock scenes
func (mb *MockBridge) handleGetScenes(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type SceneResponse struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
		Group    map[string]string      `json:"group"`
	}

	var scenes []SceneResponse
	for _, scene := range mb.Scenes {
		scenes = append(scenes, SceneResponse{
			ID:   scene.ID,
			Type: "scene",
			Metadata: map[string]interface{}{
				"name": scene.Name,
			},
			Group: map[string]string{
				"rid": "room-" + scene.ID,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   scenes,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleGetRooms returns mock rooms
func (mb *MockBridge) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type RoomResponse struct {
		ID       string            `json:"id"`
		Type     string            `json:"type"`
		Metadata map[string]string `json:"metadata"`
	}

	var rooms []RoomResponse
	for _, room := range mb.Rooms {
		rooms = append(rooms, RoomResponse{
			ID:   room.ID,
			Type: "room",
			Metadata: map[string]string{
				"name": room.Name,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   rooms,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleGetZones returns mock zones
func (mb *MockBridge) handleGetZones(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type ZoneResponse struct {
		ID       string            `json:"id"`
		Type     string            `json:"type"`
		Metadata map[string]string `json:"metadata"`
	}

	var zones []ZoneResponse
	for _, zone := range mb.Zones {
		zones = append(zones, ZoneResponse{
			ID:   zone.ID,
			Type: "zone",
			Metadata: map[string]string{
				"name": zone.Name,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   zones,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleGetGroupedLights returns mock grouped lights
func (mb *MockBridge) handleGetGroupedLights(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type GroupedLightResponse struct {
		ID      string                 `json:"id"`
		Type    string                 `json:"type"`
		On      map[string]bool        `json:"on"`
		Dimming map[string]float32     `json:"dimming"`
		Owner   map[string]interface{} `json:"owner"`
	}

	var lights []GroupedLightResponse
	for _, light := range mb.GroupedLights {
		lights = append(lights, GroupedLightResponse{
			ID:   light.ID,
			Type: "grouped_light",
			On:   map[string]bool{"on": light.On},
			Dimming: map[string]float32{
				"brightness": light.Brightness,
			},
			Owner: map[string]interface{}{
				"rid":   "room-" + light.ID,
				"rtype": light.Type,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   lights,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleGetLights returns mock lights
func (mb *MockBridge) handleGetLights(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type LightResponse struct {
		ID       string             `json:"id"`
		Type     string             `json:"type"`
		On       map[string]bool    `json:"on"`
		Dimming  map[string]float32 `json:"dimming"`
		Metadata map[string]string  `json:"metadata"`
	}

	var lights []LightResponse
	for _, light := range mb.Lights {
		lights = append(lights, LightResponse{
			ID:   light.ID,
			Type: "light",
			On:   map[string]bool{"on": light.On},
			Dimming: map[string]float32{
				"brightness": light.Brightness,
			},
			Metadata: map[string]string{
				"name": light.Name,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   lights,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handleGetEntertainment returns mock entertainment configurations
func (mb *MockBridge) handleGetEntertainment(w http.ResponseWriter, r *http.Request) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	type EntertainmentResponse struct {
		ID       string            `json:"id"`
		Type     string            `json:"type"`
		Status   string            `json:"status"`
		Metadata map[string]string `json:"metadata"`
	}

	var configs []EntertainmentResponse
	for _, ent := range mb.Entertainment {
		configs = append(configs, EntertainmentResponse{
			ID:     ent.ID,
			Type:   "entertainment_configuration",
			Status: ent.Status,
			Metadata: map[string]string{
				"name": ent.Name,
			},
		})
	}

	response := map[string]interface{}{
		"errors": []interface{}{},
		"data":   configs,
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// handlePutRequest handles PUT requests (scene activation, light control)
func (mb *MockBridge) handlePutRequest(w http.ResponseWriter, r *http.Request) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Parse request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Success response
	response := map[string]interface{}{
		"errors": []interface{}{},
		"data": []map[string]interface{}{
			{
				"rid":   "success",
				"rtype": "update",
			},
		},
	}

	json.NewEncoder(w).Encode(response) //nolint:errcheck
}

// SetupDefaultScenario sets up a typical test scenario
func (mb *MockBridge) SetupDefaultScenario() {
	mb.AddScene("scene-1", "Relax", "Living Room")
	mb.AddScene("scene-2", "Bright", "Kitchen")
	mb.AddScene("scene-3", "Concentrate", "Office")

	mb.AddGroupedLight("room-1", "Living Room", "room")
	mb.AddGroupedLight("room-2", "Kitchen", "room")
	mb.AddGroupedLight("zone-1", "TV Area", "zone")

	mb.AddLight("light-1", "Ceiling Light 1")
	mb.AddLight("light-2", "Ceiling Light 2")
	mb.AddLight("light-3", "Floor Lamp")

	mb.AddRoom("room-1", "Living Room")
	mb.AddRoom("room-2", "Kitchen")
	mb.AddRoom("room-3", "Office")

	mb.AddZone("zone-1", "TV Area")
}
