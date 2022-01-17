package igdb

import (
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/magiconair/properties"
	"log"
	"os"
	"strconv"
	"strings"
	"vg-cover-screen-saver-go/internal/app/domain"
)

var (
	errorLogger *log.Logger
	warnLogger  *log.Logger
	infoLogger  *log.Logger
)

func init() {
	logFile, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	errorLogger = log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	warnLogger = log.New(logFile, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
}

type twitchTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type igdbGame struct {
	Id          int    `json:"id"`
	Artworks    []int  `json:"artworks"`
	Screenshots []int  `json:"screenshots"`
	Name        string `json:"name"`
}

func GetGameArtworks(clientGame domain.ClientGame, props properties.Properties) ([]domain.IgdbGameArtwork, error) {
	authToken, err := getAuthToken(props)
	if err != nil {
		return nil, err
	}

	game, gameError := getIgdbGame(clientGame, props, authToken)
	if gameError != nil {
		return nil, gameError
	}

	if game == nil {
		return nil, nil
	}

	fmt.Println("Fetching IGDB artworks ...")
	var igdbArtworks []domain.IgdbGameArtwork

	covers, coverError := getIgdbArtworkByGameId(domain.Cover, game.Id, props, authToken)
	if coverError == nil {
		igdbArtworks = append(igdbArtworks, *covers...)
	} else {
		fmt.Println("Fetching IGDB artworks failed! Failed on covers")
		return nil, coverError
	}

	artworks, artworkError := getIgdbArtworkByGameId(domain.Artwork, game.Id, props, authToken)
	if artworkError == nil {
		igdbArtworks = append(igdbArtworks, *artworks...)
	} else {
		fmt.Println("Fetching IGDB artworks failed! Failed on game artworks")
		return nil, artworkError
	}

	artworks2, artworkError2 := getIgdbArtworksFromIds(domain.Artwork, game.Artworks, props, authToken)
	if artworkError2 == nil {
		igdbArtworks = append(igdbArtworks, artworks2...)
	} else {
		fmt.Println("Fetching IGDB artworks failed! Failed on game artworks by ID")
		return nil, artworkError2
	}

	screenShots, screenShotError := getIgdbArtworkByGameId(domain.ScreenShot, game.Id, props, authToken)
	if screenShotError == nil {
		igdbArtworks = append(igdbArtworks, *screenShots...)
	} else {
		fmt.Println("Fetching IGDB artworks failed! Failed on game screen shots")
		return nil, screenShotError
	}

	screenShots2, screenShotError2 := getIgdbArtworksFromIds(domain.ScreenShot, game.Screenshots, props, authToken)
	if screenShotError2 == nil {
		igdbArtworks = append(igdbArtworks, screenShots2...)
	} else {
		fmt.Println("Fetching IGDB artworks failed! Failed on game screen shots by ID")
		return nil, screenShotError2
	}

	fmt.Println("Fetching IGDB artworks success!")
	return igdbArtworks, nil
}

func getAuthToken(props properties.Properties) (string, error) {
	twitchAuthClient := resty.New()
	twitchAuthResp, twitchAuthError := twitchAuthClient.R().
		SetQueryParams(map[string]string{
			"client_id":     props.MustGet("igdb.client.id"),
			"client_secret": props.MustGet("igdb.client.secret"),
			"grant_type":    "client_credentials",
		}).
		SetResult(twitchTokenResponse{}).
		Post("https://id.twitch.tv/oauth2/token")
	if twitchAuthError != nil {
		return "", twitchAuthError
	}
	authToken := twitchAuthResp.Result().(*twitchTokenResponse).AccessToken
	return authToken, nil
}

func getIgdbGame(clientGame domain.ClientGame, props properties.Properties, authToken string) (*igdbGame, error) {
	fmt.Println("Fetching IGDB game....")
	igdbClient := resty.New()
	body := "search \"" + strings.ReplaceAll(clientGame.Name, "®", "") + "\"; fields name,artworks,screenshots;"
	igdbResp, errIgdb := igdbClient.R().
		EnableTrace().
		SetAuthToken(authToken).
		SetHeader("Client-ID", props.MustGet("igdb.client.id")).
		SetBody(body).
		SetResult([]igdbGame{}).
		Post("https://api.igdb.com/v4/games/")
	if errIgdb == nil || igdbResp.StatusCode() < 199 || igdbResp.StatusCode() > 299 {
		fmt.Println("Fetching IGDB games success!")
		igdbResults := igdbResp.Result().(*[]igdbGame)
		topRankGame := findClosesResultMatch(clientGame, igdbResults)
		return topRankGame, nil
	} else {
		fmt.Println("Fetching IGDB games failed!")
		if errIgdb == nil {
			return nil, errors.New("Fetching IGDB games failed! Response Code: " + strconv.Itoa(igdbResp.StatusCode()) + " Response Message: " + igdbResp.String())
		}
		return nil, errIgdb
	}
}

/**
IGDB returns a list of results with titles names close to the search string. That means the search will return
sequels and closely names titles. Here we do a fuzzy name match to get the one title we really need.
*/
func findClosesResultMatch(clientGame domain.ClientGame, igdbResults *[]igdbGame) *igdbGame {
	if len(*igdbResults) == 0 {
		return nil
	}

	// No need to search if only one result
	if len(*igdbResults) == 1 {
		return &(*igdbResults)[0]
	}

	var igdbGameNames []string
	for _, igdbGameResult := range *igdbResults {
		igdbGameNames = append(igdbGameNames, strings.ToUpper(igdbGameResult.Name))
	}

	// fuzzy search is case-sensitive so comparison values are normalized to upper case
	fuzzyNames := fuzzy.RankFind(strings.ToUpper(clientGame.Name), igdbGameNames)
	var topRankName string
	if fuzzyNames.Len() > 0 {
		topRankName = fuzzyNames[0].Source
	} else {
		topRankName = igdbGameNames[0]
	}

	var topRankGame igdbGame
	for _, igdbGameResult := range *igdbResults {
		if strings.ToUpper(igdbGameResult.Name) == topRankName {
			topRankGame = igdbGameResult
			break
		}
	}
	return &topRankGame
}

func getIgdbArtworksFromIds(artType domain.ArtworkType, artworkIds []int, props properties.Properties, authToken string) ([]domain.IgdbGameArtwork, error) {
	var artworks []domain.IgdbGameArtwork
	for _, artworkId := range artworkIds {
		igdbArtworks, artworkError := getIgdbArtworkById(artType, artworkId, props, authToken)
		if artworkError == nil {
			for _, artwork := range *igdbArtworks {
				artworks = append(artworks, artwork)
			}
		} else {
			return nil, artworkError
		}
	}
	return artworks, nil
}

func getIgdbArtworkByGameId(artType domain.ArtworkType, gameId int, props properties.Properties, authToken string) (*[]domain.IgdbGameArtwork, error) {
	query := "fields image_id; where game = " + strconv.Itoa(gameId) + " & animated = false;"
	return getIgdbArtwork(artType, query, props, authToken)
}

func getIgdbArtworkById(artType domain.ArtworkType, artworkId int, props properties.Properties, authToken string) (*[]domain.IgdbGameArtwork, error) {
	query := "fields image_id; where id = " + strconv.Itoa(artworkId) + ";"
	return getIgdbArtwork(artType, query, props, authToken)
}

func getIgdbArtwork(artType domain.ArtworkType, query string, props properties.Properties, authToken string) (*[]domain.IgdbGameArtwork, error) {
	igdbClient := resty.New()
	igdbArtworksResp, errIgdbArtworks := igdbClient.R().
		EnableTrace().
		SetAuthToken(authToken).
		SetHeader("Client-ID", props.MustGet("igdb.client.id")).
		SetBody(query).
		SetResult([]domain.IgdbGameArtwork{}).
		Post("https://api.igdb.com/v4/" + artType.String())
	if errIgdbArtworks == nil {
		artworksResp := igdbArtworksResp.Result().(*[]domain.IgdbGameArtwork)
		return artworksResp, nil
	} else {
		return nil, errIgdbArtworks
	}
}
