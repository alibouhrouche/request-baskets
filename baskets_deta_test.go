package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetabase_Create(t *testing.T) {
	name := "test1"
	db := NewDetabase()
	defer db.Release()

	auth, err := db.Create(name, BasketConfig{Capacity: 20})
	if assert.NoError(t, err) {
		assert.NotEmpty(t, auth.Token, "basket token may not be empty")
		assert.False(t, len(auth.Token) < 30, "weak basket token: %v", auth.Token)
	}
}

func TestDetabase_Create_NameConflict(t *testing.T) {
	name := "test2"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 20})
	auth, err := db.Create(name, BasketConfig{Capacity: 20})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "'"+name+"'", "error is not detailed enough")
		assert.Empty(t, auth.Token, "basket token is not expected")
	}
}

func TestDetabase_Get(t *testing.T) {
	name := "test3"
	db := NewDetabase()
	defer db.Release()

	auth, err := db.Create(name, BasketConfig{Capacity: 16})
	assert.NoError(t, err)

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		assert.True(t, basket.Authorize(auth.Token), "basket authorization has failed")
		assert.Equal(t, 16, basket.Config().Capacity, "wrong capacity")
	}
}

func TestDetabase_Get_NotFound(t *testing.T) {
	name := "test4"
	db := NewDetabase()
	defer db.Release()

	basket := db.Get(name)
	assert.Nil(t, basket, "basket with name: %v is not expected", name)
}

func TestDetabase_Delete(t *testing.T) {
	name := "test5"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 10})
	assert.NotNil(t, db.Get(name), "basket with name: %v is expected", name)

	db.Delete(name)
	assert.Nil(t, db.Get(name), "basket with name: %v is not expected", name)
}

func TestDetabase_Delete_Multi(t *testing.T) {
	name := "test6"
	db := NewDetabase()
	defer db.Release()

	config := BasketConfig{Capacity: 10}
	for i := 0; i < 10; i++ {
		db.Create(fmt.Sprintf("test%v", i), config)
	}

	assert.NotNil(t, db.Get(name), "basket with name: %v is expected", name)
	assert.Equal(t, 10, db.Size(), "wrong database size")

	db.Delete(name)

	assert.Nil(t, db.Get(name), "basket with name: %v is not expected", name)
	assert.Equal(t, 9, db.Size(), "wrong database size")
}

func TestDetabase_Size(t *testing.T) {
	db := NewDetabase()
	defer db.Release()

	config := BasketConfig{Capacity: 15}
	for i := 0; i < 25; i++ {
		db.Create(fmt.Sprintf("test%v", i), config)
	}

	assert.Equal(t, 25, db.Size(), "wrong database size")
}

func TestDetaBasket_Add(t *testing.T) {
	name := "test101"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 20})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// add 1st HTTP request
		content := "{ \"user\": \"tester\", \"age\": 24 }"
		data := basket.Add(createTestPOSTRequest(
			fmt.Sprintf("http://localhost/%v/demo?name=abc&ver=12", name), content, "application/json"))

		assert.Equal(t, 1, basket.Size(), "wrong basket size")

		// detailed http.Request to RequestData tests should be covered by test of ToRequestData function
		assert.Equal(t, content, data.Body, "wrong body")
		assert.Equal(t, int64(len(content)), data.ContentLength, "wrong content length")

		// add 2nd HTTP request
		basket.Add(createTestPOSTRequest(fmt.Sprintf("http://localhost/%v/demo", name), "Hellow world", "text/plain"))
		assert.Equal(t, 2, basket.Size(), "wrong basket size")
	}
}

func TestDetaBasket_Add_ExceedLimit(t *testing.T) {
	name := "test102"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 10})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// fill basket
		for i := 0; i < 35; i++ {
			basket.Add(createTestPOSTRequest(
				fmt.Sprintf("http://localhost/%v/demo", name), fmt.Sprintf("test%v", i), "text/plain"))
		}
		assert.Equal(t, 10, basket.Size(), "wrong basket size")
	}
}

func TestDetaBasket_Clear(t *testing.T) {
	name := "test103"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 20})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// fill basket
		for i := 0; i < 15; i++ {
			basket.Add(createTestPOSTRequest(
				fmt.Sprintf("http://localhost/%v/demo", name), fmt.Sprintf("test%v", i), "text/plain"))
		}
		assert.Equal(t, 15, basket.Size(), "wrong basket size")

		// clean basket
		basket.Clear()
		assert.Equal(t, 0, basket.Size(), "wrong basket size, empty basket is expected")
	}
}

func TestDetaBasket_Update_Shrink(t *testing.T) {
	name := "test104"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 30})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// fill basket
		for i := 0; i < 25; i++ {
			basket.Add(createTestPOSTRequest(
				fmt.Sprintf("http://localhost/%v/demo", name), fmt.Sprintf("test%v", i), "text/plain"))
		}
		assert.Equal(t, 25, basket.Size(), "wrong basket size")

		// update config with lower capacity
		config := basket.Config()
		config.Capacity = 12
		basket.Update(config)
		assert.Equal(t, config.Capacity, basket.Size(), "wrong basket size")
	}
}

func TestDetaBasket_GetRequests(t *testing.T) {
	name := "test105"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 25})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// fill basket
		for i := 1; i <= 35; i++ {
			basket.Add(createTestPOSTRequest(
				fmt.Sprintf("http://localhost/%v/demo?id=%v", name, i), fmt.Sprintf("req%v", i), "text/plain"))
		}
		assert.Equal(t, 25, basket.Size(), "wrong basket size")

		// Get and validate last 10 requests
		page1 := basket.GetRequests(10, 0)
		assert.True(t, page1.HasMore, "expected more requests")
		assert.Len(t, page1.Requests, 10, "wrong page size")
		assert.Equal(t, 25, page1.Count, "wrong requests count")
		assert.Equal(t, 35, page1.TotalCount, "wrong requests total count")
		assert.Equal(t, "req35", page1.Requests[0].Body, "last request #35 is expected at index #0")

		// Get and validate 10 requests, skip 20
		page3 := basket.GetRequests(10, 20)
		assert.False(t, page3.HasMore, "no more requests are expected")
		assert.Len(t, page3.Requests, 5, "wrong page size")
		assert.Equal(t, 25, page3.Count, "wrong requests count")
		assert.Equal(t, 35, page3.TotalCount, "wrong requests total count")
		assert.Equal(t, "req15", page3.Requests[0].Body, "request #15 is expected at index #0")
	}
}

func TestDetaBasket_FindRequests(t *testing.T) {
	name := "test106"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 100})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// fill basket
		for i := 1; i <= 30; i++ {
			r := createTestPOSTRequest(fmt.Sprintf("http://localhost/%v?id=%v", name, i), fmt.Sprintf("req%v", i), "text/plain")
			r.Header.Add("HeaderId", fmt.Sprintf("header%v", i))
			if i <= 10 {
				r.Header.Add("ChocoPie", "yummy")
			}
			if i <= 20 {
				r.Header.Add("Muffin", "tasty")
			}
			basket.Add(r)
		}
		assert.Equal(t, 30, basket.Size(), "wrong basket size")

		// search everywhere
		s1 := basket.FindRequests("req1", "any", 30, 0)
		assert.False(t, s1.HasMore, "no more results are expected")
		assert.Len(t, s1.Requests, 11, "wrong number of found requests")
		for _, r := range s1.Requests {
			assert.Contains(t, r.Body, "req1", "incorrect request among results")
		}

		// search everywhere (limited output)
		s2 := basket.FindRequests("req2", "any", 5, 5)
		assert.True(t, s2.HasMore, "more results are expected")
		assert.Len(t, s2.Requests, 5, "wrong number of found requests")

		// search in body (positive)
		assert.Len(t, basket.FindRequests("req3", "body", 100, 0).Requests, 2, "wrong number of found requests")
		// search in body (negative)
		assert.Empty(t, basket.FindRequests("yummy", "body", 100, 0).Requests, "found unexpected requests")

		// search in headers (positive)
		assert.Len(t, basket.FindRequests("yummy", "headers", 100, 0).Requests, 10, "wrong number of found requests")
		assert.Len(t, basket.FindRequests("tasty", "headers", 100, 0).Requests, 20, "wrong number of found requests")
		// search in headers (negative)
		assert.Empty(t, basket.FindRequests("req1", "headers", 100, 0).Requests, "found unexpected requests")

		// search in query (positive)
		assert.Len(t, basket.FindRequests("id=1", "query", 100, 0).Requests, 11, "wrong number of found requests")
		// search in query (negative)
		assert.Empty(t, basket.FindRequests("tasty", "query", 100, 0).Requests, "found unexpected requests")
	}
}

func TestDetaBasket_SetResponse(t *testing.T) {
	name := "test107"
	method := "POST"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 20})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// Ensure no response
		assert.Nil(t, basket.GetResponse(method))

		// Set response
		basket.SetResponse(method, ResponseConfig{Status: 201, Body: "{ 'message' : 'created' }"})
		// Get and validate
		response := basket.GetResponse(method)
		if assert.NotNil(t, response, "response for method: %v is expected", method) {
			assert.Equal(t, 201, response.Status, "wrong HTTP response status")
			assert.Equal(t, "{ 'message' : 'created' }", response.Body, "wrong HTTP response body")
			assert.False(t, response.IsTemplate, "template is not expected")
		}
	}
}

func TestDetaBasket_SetResponse_Update(t *testing.T) {
	name := "test108"
	method := "GET"
	db := NewDetabase()
	defer db.Release()

	db.Create(name, BasketConfig{Capacity: 20})

	basket := db.Get(name)
	if assert.NotNil(t, basket, "basket with name: %v is expected", name) {
		// Set response
		basket.SetResponse(method, ResponseConfig{Status: 200, Body: ""})
		// Update response
		basket.SetResponse(method, ResponseConfig{Status: 200, Body: "welcome", IsTemplate: true})
		// Get and validate
		response := basket.GetResponse(method)
		if assert.NotNil(t, response, "response for method: %v is expected", method) {
			assert.Equal(t, 200, response.Status, "wrong HTTP response status")
			assert.Equal(t, "welcome", response.Body, "wrong HTTP response body")
			assert.True(t, response.IsTemplate, "template is expected")
		}
	}
}
