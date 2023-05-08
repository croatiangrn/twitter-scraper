package twitterscraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const bearerToken string = "AAAAAAAAAAAAAAAAAAAAAPYXBAAAAAAACLXUNDekMxqa8h%2F40K4moUkGsoc%3DTYfbDKbT3jJPCEVnMYqilB28NHfOPqkca3qaAxGfsyKCs0wRbw"

type ResponseAPIHeaders struct {
	XRateLimitReset     int64
	XRateLimitLimit     int
	XRateLimitRemaining int
}

type RequestAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (r *RequestAPIError) Error() string {
	return fmt.Sprintf("Code: %d; Message: %s", r.Code, r.Message)
}

// RequestAPI get JSON from frontend API and decodes it
func (s *Scraper) RequestAPI(req *http.Request, target interface{}) (*ResponseAPIHeaders, error) {
	s.wg.Wait()
	if s.delay > 0 {
		defer func() {
			s.wg.Add(1)
			go func() {
				time.Sleep(time.Second * time.Duration(s.delay))
				s.wg.Done()
			}()
		}()
	}

	if !s.IsGuestToken() || s.guestCreatedAt.Before(time.Now().Add(-time.Hour*3)) {
		err := s.GetGuestToken()
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Authorization", "Bearer "+s.bearerToken)
	req.Header.Set("X-Guest-Token", s.guestToken)

	for _, cookie := range s.client.Jar.Cookies(req.URL) {
		if cookie.Name == "ct0" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
			break
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// private profiles return forbidden, but also data
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
		content, _ := io.ReadAll(resp.Body)

		reqErr := RequestAPIError{}

		if err := json.Unmarshal(content, &reqErr); err != nil {
			return nil, errors.New(string(content))
		}

		return nil, &reqErr
	}

	// Print all response headers
	allowedHeaders := map[string]struct{}{
		"x-rate-limit-limit":     {},
		"x-rate-limit-reset":     {},
		"x-rate-limit-remaining": {},
	}

	requestAPIHeaders := ResponseAPIHeaders{}
	for header, values := range resp.Header {
		if _, ok := allowedHeaders[strings.ToLower(header)]; ok {
			switch strings.ToLower(header) {
			case "x-rate-limit-limit":
				if len(values) > 0 {
					limit, limitErr := strconv.Atoi(values[0])
					if limitErr != nil {
						return nil, err
					}
					requestAPIHeaders.XRateLimitLimit = limit
				}
			case "x-rate-limit-reset":
				if len(values) > 0 {
					limit, limitErr := strconv.ParseInt(values[0], 10, 64)
					if limitErr != nil {
						return nil, err
					}
					requestAPIHeaders.XRateLimitReset = limit
				}
			case "x-rate-limit-remaining":
				if len(values) > 0 {
					limit, limitErr := strconv.Atoi(values[0])
					if limitErr != nil {
						return nil, err
					}
					requestAPIHeaders.XRateLimitRemaining = limit
				}
			}
		}
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &requestAPIHeaders, json.Unmarshal(b, target)
}

// GetGuestToken from Twitter API
func (s *Scraper) GetGuestToken() error {
	req, err := http.NewRequest("POST", "https://api.twitter.com/1.1/guest/activate.json", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.bearerToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response status %s: %s", resp.Status, body)
	}

	var jsn map[string]interface{}
	if err := json.Unmarshal(body, &jsn); err != nil {
		return err
	}
	var ok bool
	if s.guestToken, ok = jsn["guest_token"].(string); !ok {
		return fmt.Errorf("guest_token not found")
	}
	s.guestCreatedAt = time.Now()

	return nil
}
