package twitterscraper

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// SearchTweets returns channel with tweets for a given search query
func (s *Scraper) SearchTweets(ctx context.Context, query string, maxTweetsNbr int) <-chan *TweetResult {
	return getTweetTimeline(ctx, query, maxTweetsNbr, s.FetchSearchTweets)
}

// Deprecated: SearchTweets wrapper for default Scraper
func SearchTweets(ctx context.Context, query string, maxTweetsNbr int) <-chan *TweetResult {
	return defaultScraper.SearchTweets(ctx, query, maxTweetsNbr)
}

// SearchProfiles returns channel with profiles for a given search query
func (s *Scraper) SearchProfiles(ctx context.Context, query string, maxProfilesNbr int) <-chan *ProfileResult {
	return getUserTimeline(ctx, query, maxProfilesNbr, s.FetchSearchProfiles)
}

// Deprecated: SearchProfiles wrapper for default Scraper
func SearchProfiles(ctx context.Context, query string, maxProfilesNbr int) <-chan *ProfileResult {
	return defaultScraper.SearchProfiles(ctx, query, maxProfilesNbr)
}

// getSearchTimeline gets results for a given search query, via the Twitter frontend API
func (s *Scraper) getSearchTimeline(query string, maxNbr int, cursor string) (*timeline, error) {
	if !s.isLogged {
		return nil, errors.New("scraper is not logged in for search")
	}

	if maxNbr > 50 {
		maxNbr = 50
	}

	req, err := http.NewRequest("GET", "https://twitter.com/i/api/2/search/adaptive.json", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("include_profile_interstitial_type", "1")
	q.Add("include_blocking", "1")
	q.Add("include_blocked_by", "1")
	q.Add("include_followed_by", "1")
	q.Add("include_want_retweets", "1")
	q.Add("include_mute_edge", "1")
	q.Add("include_can_dm", "1")
	q.Add("include_can_media_tag", "1")
	q.Add("include_ext_has_nft_avatar", "1")
	q.Add("include_ext_is_blue_verified", "1")
	q.Add("include_ext_verified_type", "1")
	q.Add("skip_status", "1")
	q.Add("cards_platform", "Web-12")
	q.Add("include_cards", "1")
	q.Add("include_ext_alt_text", "true")
	q.Add("include_ext_limited_action_results", "false")
	q.Add("include_quote_count", "true")
	q.Add("include_reply_count", "1")
	q.Add("tweet_mode", "extended")
	q.Add("include_ext_collab_control", "true")
	q.Add("include_ext_views", "true")
	q.Add("include_entities", "true")
	q.Add("include_user_entities", "true")
	q.Add("include_ext_media_color", "true")
	q.Add("include_ext_media_availability", "true")
	q.Add("include_ext_sensitive_media_warning", "true")
	q.Add("include_ext_trusted_friends_metadata", "true")
	q.Add("send_error_codes", "true")
	q.Add("simple_quoted_tweet", "true")
	q.Add("include_tweet_replies", strconv.FormatBool(s.includeReplies))
	q.Add("ext", "mediaStats,highlightedLabel,hasNftAvatar,voiceInfo,birdwatchPivot,enrichments,superFollowMetadata,unmentionInfo,editControl,collab_control,vibe")
	q.Add("q", query)
	q.Add("count", strconv.Itoa(maxNbr))
	q.Add("query_source", "typed_query")
	q.Add("requestContext", "launch")
	q.Add("spelling_corrections", "1")
	q.Add("include_ext_edit_control", "true")
	if cursor != "" {
		q.Add("cursor", cursor)
	}

	if s.searchMode == SearchLatest {
		q.Add("pc", "0")
	} else {
		q.Add("pc", "1")
	}

	switch s.searchMode {
	case SearchLatest:
		q.Add("tweet_search_mode", "live")
	case SearchPhotos:
		q.Add("result_filter", "image")
	case SearchVideos:
		q.Add("result_filter", "video")
	case SearchUsers:
		q.Add("result_filter", "user")
	}

	req.URL.RawQuery = q.Encode()
	log.Println(req.URL.String())

	var timeline timeline
	_, err = s.RequestAPI(req, &timeline)
	if err != nil {
		return nil, err
	}
	return &timeline, nil
}

func (s *Scraper) getSearchTimelineWithResponseHeaders(query string, maxNbr int, cursor string) (*timeline, *ResponseAPIHeaders, error) {
	if !s.isLogged {
		return nil, nil, errors.New("scraper is not logged in for search")
	}

	if maxNbr > 50 {
		maxNbr = 50
	}

	req, err := s.newRequest("GET", "https://twitter.com/i/api/2/search/adaptive.json")
	if err != nil {
		return nil, nil, err
	}

	q := req.URL.Query()
	q.Add("q", query)
	q.Add("count", strconv.Itoa(maxNbr))
	q.Add("query_source", "typed_query")
	q.Add("pc", "1")
	q.Add("requestContext", "launch")
	q.Add("spelling_corrections", "1")
	q.Add("include_ext_edit_control", "true")
	if cursor != "" {
		q.Add("cursor", cursor)
	}
	switch s.searchMode {
	case SearchLatest:
		q.Add("f", "live")
	case SearchPhotos:
		q.Add("result_filter", "image")
	case SearchVideos:
		q.Add("result_filter", "video")
	case SearchUsers:
		q.Add("result_filter", "user")
	}

	req.URL.RawQuery = q.Encode()

	var timeline timeline
	responseHeaders, reqApiErr := s.RequestAPI(req, &timeline)
	if reqApiErr != nil {
		return nil, nil, reqApiErr
	}
	return &timeline, responseHeaders, nil
}

// FetchSearchTweets gets tweets for a given search query, via the Twitter frontend API
func (s *Scraper) FetchSearchTweets(query string, maxTweetsNbr int, cursor string) ([]*Tweet, string, error) {
	timeline, err := s.getSearchTimeline(query, maxTweetsNbr, cursor)
	if err != nil {
		return nil, "", err
	}
	tweets, nextCursor := timeline.parseTweets()
	return tweets, nextCursor, nil
}

func (s *Scraper) FetchSearchTweetsWithResponseHeaders(query string, maxTweetsNbr int, cursor string) ([]*Tweet, string, *ResponseAPIHeaders, error) {
	timeline, responseHeaders, err := s.getSearchTimelineWithResponseHeaders(query, maxTweetsNbr, cursor)
	if err != nil {
		return nil, "", nil, err
	}
	tweets, nextCursor := timeline.parseTweets()
	return tweets, nextCursor, responseHeaders, nil
}

// FetchSearchProfiles gets users for a given search query, via the Twitter frontend API
func (s *Scraper) FetchSearchProfiles(query string, maxProfilesNbr int, cursor string) ([]*Profile, string, error) {
	timeline, err := s.getSearchTimeline(query, maxProfilesNbr, cursor)
	if err != nil {
		return nil, "", err
	}
	users, nextCursor := timeline.parseUsers()
	return users, nextCursor, nil
}
