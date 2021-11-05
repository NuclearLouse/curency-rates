package service

import (
	"context"
	"currency-rates/internal/datastructs"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

/*

API: https://www.lb.lt/webservices/FxRates/en/
tp = LT or EUR

1. getCurrencyList: List of ISO 4217 currencies
https://www.lb.lt/webservices/FxRates/FxRates.asmx/getCurrencyList

2. getCurrentFxRates: Last available currency exchange rates
https://www.lb.lt/webservices/FxRates/FxRates.asmx/getCurrentFxRates?tp=string

3. getFxRates: Currency exchange rates at specified date
https://www.lb.lt/webservices/FxRates/FxRates.asmx/getFxRates?tp=string&dt=string

4. getFxRatesForCurrency: Exchange rates for specified currency at date interval
https://www.lb.lt/webservices/FxRates/FxRates.asmx/getFxRatesForCurrency?tp=string&ccy=string&dtFrom=string&dtTo=string

*/

func (s *Service) urlRequest(request string, values ...map[string]string) string {
	val := url.Values{}
	if len(values) != 0 {
		for key, value := range values[0] {
			val.Set(key, value)
		}
	}
	u := url.URL{
		Scheme:   s.ini.Section("api").Key("protocol").MustString("https"),
		Host:     s.ini.Section("api").Key("host").String(),
		Path:     s.ini.Section("api").Key("path").String() + "/" + request,
		RawQuery: val.Encode(),
	}
	return u.String()
}

func (s *Service) curencyList() (map[string]string, error) {
	url := s.urlRequest(
		s.ini.Section("api.request").Key("currency_list").String(),
		nil,
	)
	s.log.Trace("request: ", url)
	res, err := s.requestAPI(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	s.log.Trace("response: ", res.Status)

	var result datastructs.CurrencyList
	if err := xml.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode xml:%s", err)
	}

	list := make(map[string]string)
	for _, r := range result.Currency {
		for _, n := range r.Name {
			if n.Lang == "EN" {
				list[r.Code] = n.Text
			}
		}
	}
	return list, nil
}

func (s *Service) currentFxRates() (map[string]*datastructs.CurrencyRates, error) {
	url := s.urlRequest(
		s.ini.Section("api.request").Key("current_rates").String(),
		map[string]string{
			s.ini.Section("api.request.value").Key("rate_type").String(): "",
		},
	)
	s.log.Trace("request: ", url)
	res, err := s.requestAPI(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	s.log.Trace("response: ", res.Status)

	var result datastructs.FxRates
	if err := xml.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode xml:%s", err)
	}

	rates := make(map[string]*datastructs.CurrencyRates)
	for _, r := range result.FxRate {
		date, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			date = time.Time{}
		}
		for _, c := range r.Curency {
			amount, err := strconv.ParseFloat(c.Amount, 64)
			if err != nil {
				amount = 0
			}
			rates[c.Code] = &datastructs.CurrencyRates{
				Date:       date,
				Code:       c.Code,
				Proportion: amount,
			}
		}
	}
	return rates, nil
}

func (s *Service) requestAPI(url string) (*http.Response, error) {
	timeoutResponse := time.Duration(s.TimeoutResponse) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeoutResponse)
	defer cancel()

	chanRespErr := make(chan error)
	var res *http.Response
	go func() {
		var err error
		res, err = func() (*http.Response, error) {
			client := http.Client{
				Timeout: time.Duration(s.TimeoutRequest) * time.Second,
			}

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", req.UserAgent())
			return client.Do(req)
		}()
		chanRespErr <- err
	}()
	
	select {
	case <-ctx.Done():
		return nil, errors.New("request timed out")
	case err := <-chanRespErr:
		return res, err
	}
}
