package steam

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/go-resty/resty/v2"
	"github.com/magiconair/properties"
	"github.com/mitchellh/mapstructure"
	"strconv"
	"strings"
	"time"
	"vg-cover-screen-saver-go/internal/app/domain"
)

type userOwnedGameResponse struct {
	Response userOwnedGameList `json:"response"`
}

type userOwnedGameList struct {
	GameCount int             `json:"game_count"`
	Games     []userOwnedGame `json:"games"`
}
type userOwnedGame struct {
	AppId           int `json:"appid"`
	PlaytimeForever int `json:"playtime_forever"`
}

type gameData struct {
	Success bool `mapstructure:"success"`
	Data    game `mapstructure:"data"`
}

type game struct {
	Type        string   `mapstructure:"type"`
	Name        string   `mapstructure:"name"`
	Description string   `mapstructure:"short_description"`
	Free        bool     `mapstructure:"is_free"`
	Developers  []string `mapstructure:"developers"`
	AppId       int      `mapstructure:"steam_appid"`
}

func GetGames(clientGames []domain.ClientGame, props properties.Properties) ([]domain.ClientGame, error) {
	fmt.Println("Fetching unprocessed Steam games ...")
	userOwnedGames, userError := getUserOwnedGames(props)
	if userError != nil {
		fmt.Println("Fetching Steam games failed!")
		return nil, userError
	} else {
		unprocessedGames := findUnprocessedGames(clientGames, userOwnedGames)
		storeGames, err := getStoreGames(unprocessedGames)
		if err != nil {
			return nil, err
		}
		fmt.Println("Fetching unprocessed Steam games success!")
		return convertGames(storeGames), nil
	}
}

func getStoreGames(ownedGames []userOwnedGame) ([]game, error) {
	storeGames := make([]game, 0)
	for _, steamGame := range ownedGames {
		storeGameData, storeError := getStoreGame(steamGame.AppId)
		if storeError != nil {
			fmt.Println("Fetching unprocessed Steam games failed! Failed on game id " + strconv.Itoa(steamGame.AppId))
			return nil, storeError
		}
		fmt.Println(storeGameData.Data.Name + " " + strconv.Itoa(storeGameData.Data.AppId))
		if storeGameData.Success {
			storeGames = append(storeGames, storeGameData.Data)
		}
	}
	return storeGames, nil
}

func findUnprocessedGames(clientGames []domain.ClientGame, userOwnedGames []userOwnedGame) []userOwnedGame {
	unprocessedGames := make([]userOwnedGame, 0)
	for _, steamGame := range userOwnedGames {
		var found = false
		for _, clientGame := range clientGames {
			if clientGame.Source == domain.Steam && clientGame.SourceId == strconv.Itoa(steamGame.AppId) {
				found = true
				break
			}
		}
		if !found {
			unprocessedGames = append(unprocessedGames, steamGame)
		}
	}
	return unprocessedGames
}

func convertGames(steamGames []game) []domain.ClientGame {
	var clientGames []domain.ClientGame
	for _, steamGame := range steamGames {
		x := domain.Steam
		var clientGame domain.ClientGame
		clientGame.Name = strings.ReplaceAll(steamGame.Name, "™", "")
		clientGame.Source = x
		clientGame.SourceId = strconv.Itoa(steamGame.AppId)
		clientGame.Description = steamGame.Description
		clientGame.Developers = steamGame.Developers
		clientGames = append(clientGames, clientGame)
	}
	return clientGames
}

func getUserOwnedGames(props properties.Properties) ([]userOwnedGame, error) {
	steamUserClient := resty.New()
	userGameListResp, userGameListError := steamUserClient.R().
		EnableTrace().
		SetPathParams(map[string]string{
			"key":     props.MustGet("steam.client.key"),
			"steamId": props.MustGet("steam.client.id"),
		}).
		SetResult(userOwnedGameResponse{}).
		Get("http://api.steampowered.com/IPlayerService/GetOwnedGames/v0001/?key={key}&steamid={steamId}&format=json&include_played_free_games=false")
	if userGameListError != nil {
		return nil, userGameListError
	} else {
		ownedGames := userGameListResp.Result().(*userOwnedGameResponse).Response.Games
		return ownedGames, nil
	}
}

func getStoreGame(gameId int) (*gameData, error) {
	steamStoreGameClient := resty.
		New().
		R().
		EnableTrace().
		SetQueryParams(map[string]string{
			"appids": strconv.Itoa(gameId),
		})
	var storeGameData interface{}

	retry.Do(
		func() error {
			storeGameResp, storeGameError := steamStoreGameClient.Get("https://store.steampowered.com/api/appdetails?appids={appids}")

			if storeGameError != nil {
				return storeGameError
			} else {
				// The Steam store API returns the ID of the game as the field name and the value is the game data.
				var storeGameResponseBody map[string]interface{}
				json.Unmarshal(storeGameResp.Body(), &storeGameResponseBody)
				// TODO how to get games no longer in store? map[9870:map[success:false]] map[228020:map[success:false]]
				storeGameData = storeGameResponseBody[strconv.Itoa(gameId)]
				if storeGameData == nil {
					// TODO create specific error
					return errors.New("SteamGameData - nil")
				}

			}
			return nil
		},
		retry.Attempts(15),
		retry.DelayType(
			func(n uint, err error, config *retry.Config) time.Duration {
				return retry.BackOffDelay(n, err, config)
			}),
		retry.OnRetry(
			func(retryCount uint, err error) {
				fmt.Printf("Warn: retry count = %d error = %s\n", retryCount, err)
			}),
	)

	steamGameData := &gameData{}
	decodeError := mapstructure.Decode(storeGameData, steamGameData)
	if decodeError != nil {
		return nil, decodeError
	}
	return steamGameData, nil
}
