package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	client  = &http.Client{Timeout: time.Second}
)

type User struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Age    int    `json:"age"`
	About  string `json:"about"`
	Gender string `json:"gender"`
}

type SearchResponse struct {
	Users    []User
	NextPage bool
}

type SearchErrorResponse struct {
	Error string `json:"error"`
}

const (
	OrderByDesc = -1
	OrderByAsIs = 0
	OrderByAsc  = 1

	ErrorBadOrderField = `OrderField invalid`
	MaxLimit           = 25
)

type SearchRequest struct {
	Limit      int
	Offset     int
	Query      string
	OrderField string
	// -1 по убыванию, 0 как встретилось, 1 по возрастанию
	OrderBy int
}

type SearchClient struct {
	// токен, по которому происходит авторизация на внешней системе, уходит туда через хедер
	AccessToken string
	// урл внешней системы, куда идти
	URL string
}

// FindUsers отправляет запрос во внешнюю систему, которая непосредственно ищет пользоваталей
func (srv *SearchClient) FindUsers(req SearchRequest) (*SearchResponse, error) {

	searcherParams := url.Values{}

	if req.Limit < 0 {
		return nil, fmt.Errorf("limit must be > 0")
	}
	if req.Limit > MaxLimit {
		req.Limit = MaxLimit
	}
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be > 0")
	}

	//нужно для получения следующей записи, на основе которой мы скажем - можно показать переключатель следующей страницы или нет
	//req.Limit++

	searcherParams.Add("limit", strconv.Itoa(req.Limit))
	searcherParams.Add("offset", strconv.Itoa(req.Offset))
	searcherParams.Add("query", req.Query)
	searcherParams.Add("order_field", req.OrderField)
	searcherParams.Add("order_by", strconv.Itoa(req.OrderBy))

	searcherReq, err := http.NewRequest("GET", srv.URL+"?"+searcherParams.Encode(), nil)
	searcherReq.Header.Add("AccessToken", srv.AccessToken)
	resp, err := client.Do(searcherReq)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return nil, fmt.Errorf("timeout for %s", searcherParams.Encode())
		}
		return nil, fmt.Errorf("unknown error %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	// fmt.Println("FindUsers resp.Status", resp.Status)
	// fmt.Println("FindUsers body", body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("Bad AccessToken")
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("SearchServer fatal error")
	case http.StatusBadRequest:
		errResp := SearchErrorResponse{}
		err = json.Unmarshal(body, &errResp)
		if err != nil {
			return nil, fmt.Errorf("cant unpack error json: %s", err)
		}
		if errResp.Error == ErrorBadOrderField {
			return nil, fmt.Errorf("OrderField %s invalid", req.OrderField)
		}
		return nil, fmt.Errorf("unknown bad request error: %s", errResp.Error)
	}

	result := SearchResponse{}
	err = json.Unmarshal(body, &result.Users)
	if err != nil {
		return nil, fmt.Errorf("cant unpack result json: %s", err)
	}

	//if len(result.Users) > req.Limit {
	//	result.NextPage = true
	//}

	// fmt.Printf("%+v", data)

	return &result, err
}
