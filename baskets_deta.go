package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/deta/deta-go/deta"
	"github.com/deta/deta-go/service/base"
)

// DbTypedeta defines name of detabase storage
const DbTypeDeta = "deta"

/// Basket interface ///

type detaBasket struct {
	sync.RWMutex
	base       *base.Base
	Key        string                     `json:"key"` // json struct tag 'key' used to denote the key
	Token      string                     `json:"token"`
	BConfig    BasketConfig               `json:"config"`
	Requests   []*RequestData             `json:"requests"`
	TotalCount int                        `json:"totalCount"`
	Responses  map[string]*ResponseConfig `json:"responses"`
}

func (basket *detaBasket) applyLimit() {
	// Keep requests up to specified capacity
	if len(basket.Requests) > basket.BConfig.Capacity {
		basket.Requests = basket.Requests[:basket.BConfig.Capacity]
		basket.base.Update(basket.Key, base.Updates{
			"requests": basket.Requests,
		})
	}
}

func (basket *detaBasket) Config() BasketConfig {
	return basket.BConfig
}

func (basket *detaBasket) Update(config BasketConfig) {
	basket.Lock()
	defer basket.Unlock()

	basket.BConfig = config
	basket.applyLimit()

	basket.base.Update(basket.Key, base.Updates{
		"config": config,
	})
}

func (basket *detaBasket) Authorize(token string) bool {
	return token == basket.Token
}

func (basket *detaBasket) GetResponse(method string) *ResponseConfig {
	basket.Lock()
	defer basket.Unlock()

	if response, exists := basket.Responses[method]; exists {
		return response
	}

	return nil
}

func (basket *detaBasket) SetResponse(method string, response ResponseConfig) {
	basket.Lock()
	defer basket.Unlock()

	basket.Responses[method] = &response
	basket.base.Update(basket.Key, base.Updates{
		fmt.Sprint("responses.", method): response,
	})
}

func (basket *detaBasket) Add(req *http.Request) *RequestData {
	basket.Lock()
	defer basket.Unlock()

	data := ToRequestData(req)
	// insert in front of collection
	basket.Requests = append([]*RequestData{data}, basket.Requests...)

	// keep total number of all collected requests
	basket.TotalCount++
	// apply limits according to basket capacity
	basket.applyLimit()

	basket.base.Update(basket.Key, base.Updates{
		"requests":   basket.base.Util.Prepend(data),
		"totalCount": basket.base.Util.Increment(1),
	})

	return data
}

func (basket *detaBasket) Clear() {
	basket.Lock()
	defer basket.Unlock()

	// reset collected requests and total counter
	basket.Requests = make([]*RequestData, 0, basket.BConfig.Capacity)
	// basket.totalCount = 0 // reset total stats
	basket.base.Update(basket.Key, base.Updates{
		"requests": basket.Requests,
	})
}

func (basket *detaBasket) Size() int {
	return len(basket.Requests)
}

func (basket *detaBasket) GetRequests(max int, skip int) RequestsPage {
	basket.RLock()
	defer basket.RUnlock()

	size := basket.Size()
	last := skip + max

	requestsPage := RequestsPage{
		Count:      size,
		TotalCount: basket.TotalCount,
		HasMore:    last < size}

	if skip < size {
		if last > size {
			last = size
		}
		requestsPage.Requests = basket.Requests[skip:last]
	}

	return requestsPage
}

func (basket *detaBasket) FindRequests(query string, in string, max int, skip int) RequestsQueryPage {
	basket.RLock()
	defer basket.RUnlock()

	result := make([]*RequestData, 0, max)
	skipped := 0

	for index, request := range basket.Requests {
		// filter
		if request.Matches(query, in) {
			if skipped < skip {
				skipped++
			} else {
				result = append(result, request)
			}
		}

		// early exit
		if len(result) == max {
			return RequestsQueryPage{Requests: result, HasMore: index < len(basket.Requests)-1}
		}
	}

	// whole basket is scanned through
	return RequestsQueryPage{Requests: result, HasMore: false}
}

/// BasketsDatabase interface ///

type detaDatabase struct {
	sync.RWMutex
	base *base.Base
	keys []string
}
type basketData struct {
	Key        string                     `json:"key"`
	Token      string                     `json:"token"`
	Config     BasketConfig               `json:"config"`
	Requests   []*RequestData             `json:"requests"`
	TotalCount int                        `json:"totalCount"`
	Responses  map[string]*ResponseConfig `json:"responses"`
}

func (db *detaDatabase) Create(name string, config BasketConfig) (BasketAuth, error) {
	auth := BasketAuth{}
	token, err := GenerateToken()
	if err != nil {
		return auth, fmt.Errorf("failed to generate token: %s", err)
	}

	db.Lock()
	defer db.Unlock()

	data := &basketData{
		Key:        name,
		Token:      token,
		Requests:   make([]*RequestData, 0, config.Capacity),
		Config:     config,
		Responses:  make(map[string]*ResponseConfig),
		TotalCount: 0,
	}

	_, err = db.base.Insert(data)
	if err != nil {
		return auth, fmt.Errorf("Basket with name '%s' already exists", name)
	}
	// if
	// _, exists := db.baskets[name]
	// if exists {
	// 	return auth, fmt.Errorf("Basket with name '%s' already exists", name)
	// }

	// basket := new(detaBasket)
	// basket.token = token
	// basket.config = config
	// basket.requests = make([]*RequestData, 0, config.Capacity)
	// basket.totalCount = 0
	// basket.responses = make(map[string]*ResponseConfig)

	// db.baskets[name] = basket
	// db.names = append(db.names, name)
	// Uncomment if sorting is expected
	// sort.Strings(db.names)

	auth.Token = token

	return auth, nil
}

func (db *detaDatabase) Get(name string) Basket {
	basket := new(detaBasket)
	err := db.base.Get(name, basket)
	if err == nil {
		basket.base = db.base
		return basket
	}

	log.Printf("[warn] no basket found: %s", name)
	return nil
}

func (db *detaDatabase) Delete(name string) {
	db.Lock()
	defer db.Unlock()

	_ = db.base.Delete(name)
	// delete(db.baskets, name)
	// for i, v := range db.names {
	// 	if v == name {
	// 		db.names = append(db.names[:i], db.names[i+1:]...)
	// 		break
	// 	}
	// }
}

func (db *detaDatabase) Size() int {
	return len(db.GetAllNames())
}

func (db *detaDatabase) GetAllNames() []string {
	db.RLock()
	defer db.RUnlock()
	var results []*detaBasket
	var baskets []*detaBasket
	i := &base.FetchInput{
		Q:    base.Query{},
		Dest: &baskets,
	}
	lastKey, err := db.base.Fetch(i)
	if err != nil {
		fmt.Println("failed to fetch items:", err)
	}
	results = append(results, baskets...)
	for lastKey != "" {
		// provide the last key in the fetch input
		i.LastKey = lastKey

		// fetch
		lastKey, err = db.base.Fetch(i)
		if err != nil {
			fmt.Println("failed to fetch items:", err)
		}

		// append page items to all results
		results = append(results, baskets...)
	}

	var names []string

	for _, v := range results {
		names = append(names, v.Key)
	}

	return names
}

func (db *detaDatabase) GetNames(max int, skip int) BasketNamesPage {
	if skip == 0 || db.keys == nil {
		db.keys = db.GetAllNames()
	}
	// db.RLock()
	// defer db.RUnlock()
	// var results []*detaBasket
	// var baskets []*detaBasket
	// i := &base.FetchInput{
	// 	Q:    base.Query{},
	// 	Dest: &baskets,
	// }
	// if max <= 0 {
	// 	i.Limit = max
	// }
	// lastKey, err := db.base.Fetch(i)
	// if err != nil {
	// 	fmt.Println("failed to fetch items:", err)
	// }
	// results = append(results, baskets...)
	// for lastKey != "" {
	// 	// provide the last key in the fetch input
	// 	i.LastKey = lastKey

	// 	// fetch
	// 	lastKey, err = db.base.Fetch(i)
	// 	if err != nil {
	// 		fmt.Println("failed to fetch items:", err)
	// 	}

	// 	// append page items to all results
	// 	results = append(results, baskets...)
	// }
	size := len(db.keys)
	last := skip + max

	namesPage := BasketNamesPage{
		Count:   size,
		HasMore: last < size}

	if skip < size {
		if last > size {
			last = size
		}

		namesPage.Names = db.keys[skip:last]
	}

	return namesPage
}

func (db *detaDatabase) FindNames(query string, max int, skip int) BasketNamesQueryPage {
	db.RLock()
	defer db.RUnlock()

	result := make([]string, 0, max)
	// skipped := 0

	// for index, name := range db.names {
	// 	// filter
	// 	if strings.Contains(name, query) {
	// 		if skipped < skip {
	// 			skipped++
	// 		} else {
	// 			result = append(result, name)
	// 		}
	// 	}

	// 	// early exit
	// 	if len(result) == max {
	// 		return BasketNamesQueryPage{Names: result, HasMore: index < len(db.names)-1}
	// 	}
	// }

	// whole database is scanned through
	return BasketNamesQueryPage{Names: result, HasMore: false}
}

func (db *detaDatabase) GetStats(max int) DatabaseStats {
	db.RLock()
	defer db.RUnlock()

	stats := DatabaseStats{}

	// db.keys = db.GetAllNames()
	var baskets []*detaBasket
	i := &base.FetchInput{
		Q:    base.Query{},
		Dest: &baskets,
	}
	lastKey, err := db.base.Fetch(i)
	if err != nil {
		fmt.Println("failed to fetch items:", err)
	}
	for _, v := range baskets {
		var lastRequestDate int64
		if v.Size() > 0 {
			lastRequestDate = v.GetRequests(1, 0).Requests[0].Date
		}
		stats.Collect(&BasketInfo{
			Name:               v.Key,
			RequestsCount:      v.Size(),
			RequestsTotalCount: v.TotalCount,
			LastRequestDate:    lastRequestDate,
		}, max)
	}
	// results = append(results, baskets...)
	for lastKey != "" {
		// provide the last key in the fetch input
		i.LastKey = lastKey

		// fetch
		lastKey, err = db.base.Fetch(i)
		if err != nil {
			fmt.Println("failed to fetch items:", err)
		}

		// append page items to all results
		for _, v := range baskets {
			var lastRequestDate int64
			if v.Size() > 0 {
				lastRequestDate = v.GetRequests(1, 0).Requests[0].Date
			}
			stats.Collect(&BasketInfo{
				Name:               v.Key,
				RequestsCount:      v.Size(),
				RequestsTotalCount: v.TotalCount,
				LastRequestDate:    lastRequestDate,
			}, max)
		}
	}
	// for _, name := range db.keys {
	// if basket, exists := db.baskets[name]; exists {
	// var lastRequestDate int64
	// if basket.Size() > 0 {
	// 	lastRequestDate = basket.GetRequests(1, 0).Requests[0].Date
	// }

	// stats.Collect(&BasketInfo{
	// 	Name: name,
	// 	// RequestsCount:      basket.Size(),
	// 	// RequestsTotalCount: basket.totalCount,
	// 	// LastRequestDate:    lastRequestDate
	// }, max)
	// }
	// }

	stats.UpdateAvarage()
	return stats
}

func (db *detaDatabase) Release() {
	log.Print("[info] releasing Detabase resources")
}

// NewdetaDatabase creates an instance of in-deta Baskets Database
func NewDetabase() BasketsDatabase {
	log.Print("[info] using Detabase to store baskets")
	d, err := deta.New()
	if err != nil {
		panic("failed to init new Deta instance")
	}
	db, err := base.New(d, "baskets")
	if err != nil {
		panic("failed to init new Base instance")
	}
	return &detaDatabase{base: db}
}
