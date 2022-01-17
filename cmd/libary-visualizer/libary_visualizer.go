package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"github.com/esimov/stackblur-go"
	"github.com/magiconair/properties"
	"github.com/tidwall/buntdb"
	"image"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
	"vg-cover-screen-saver-go/internal/app/domain"
	"vg-cover-screen-saver-go/internal/app/igdb"
	"vg-cover-screen-saver-go/internal/app/steam"
)

var (
	mainProps   *properties.Properties
	secretProps *properties.Properties
	errorLogger *log.Logger
	warnLogger  *log.Logger
	infoLogger  *log.Logger
)

func init() {
	rand.Seed(time.Now().UnixNano())
	logFile, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	mainProps = properties.MustLoadFile("config.properties", properties.UTF8)
	secretProps = properties.MustLoadFile("config-secret.properties", properties.UTF8)
	errorLogger = log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	warnLogger = log.New(logFile, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
	runVisualizer()
}

func runVisualizer() {

	visualizer := app.New()
	visualizerWindow := visualizer.NewWindow("Game Library Visualizer")
	visualizerWindow.Resize(fyne.NewSize(1000, 600))
	// TODO loading screen

	db, loadDbErr := loadDB()
	if loadDbErr != nil {
		errorLogger.Println("Failed to load DB: " + loadDbErr.Error())
		return
	}
	ownedGames, getGamesErr := getOwnedGames(db)
	if getGamesErr != nil {
		errorLogger.Println("Failed to fetch owned games: " + getGamesErr.Error())
		return
	}

	// TODO thread loading of images incrementally in background while displaying already and newly added images
	games, err := steam.GetGames(ownedGames, *secretProps)
	if err == nil {
		for _, gameData := range games {
			fmt.Println(gameData.Name)
			artworks, errorArtwork := igdb.GetGameArtworks(gameData, *secretProps)
			if errorArtwork == nil {
				gameData.Artworks = artworks
				// update data
				updateErr := db.Update(func(tx *buntdb.Tx) error {
					bytes, marshErr := json.Marshal(gameData)
					if marshErr != nil {
						return marshErr
					}
					_, _, setErr := tx.Set(gameData.Source.String()+gameData.SourceId, string(bytes), nil)
					return setErr
				})
				if updateErr != nil {
					return
				}
			}
		}
	}

	ownedGames, getGamesErr = getOwnedGames(db)
	if getGamesErr != nil {
		errorLogger.Println("Failed to fetch owned games: " + getGamesErr.Error())
		return
	}

	imageCoverTime, mainPropsError := strconv.Atoi(mainProps.MustGet("visualizer.image.time.seconds"))
	if mainPropsError != nil {
		errorLogger.Println("Failed to load value for visualizer.image.time.seconds. Must me numeric : " + mainPropsError.Error())
		return
	}

	showGame(ownedGames, visualizerWindow)
	go func() {
		for range time.Tick(time.Second * time.Duration(imageCoverTime)) {
			showGame(ownedGames, visualizerWindow)
		}
	}()

	visualizerWindow.ShowAndRun()
	defer func(db *buntdb.DB) {
		dbCloseErr := db.Close()
		if dbCloseErr != nil {
			errorLogger.Println("Failed to close DB properly: " + dbCloseErr.Error())
		}
	}(db)
}

func loadDB() (*buntdb.DB, error) {
	db, err := buntdb.Open("game_artwork.db")
	if err != nil {
		fmt.Println(err)
	}
	err = db.CreateIndex("source", "*", buntdb.IndexJSON("source"))
	if err != nil {
		fmt.Println(err)
	}
	err = db.CreateIndex("source_source_id", "*", buntdb.IndexJSON("source"), buntdb.IndexJSON("source-id"))
	if err != nil {
		fmt.Println(err)
	}
	return db, err
}

func getOwnedGames(db *buntdb.DB) ([]domain.ClientGame, error) {
	ownedGames := make([]domain.ClientGame, 0)
	err := db.View(func(tx *buntdb.Tx) error {
		tx.Ascend("source", func(key, value string) bool {
			game := domain.ClientGame{}
			json.Unmarshal([]byte(value), &game)
			ownedGames = append(ownedGames, game)
			return true
		})
		return nil
	})
	return ownedGames, err
}

func showGame(games []domain.ClientGame, visualizerWindow fyne.Window) {
	game := games[rand.Intn(len(games))]
	if len(game.Artworks) > 0 {
		// TODO Error handling
		artworkUrl := "https://images.igdb.com/igdb/image/upload/t_original/" + game.Artworks[0].ArtworkId + ".jpg"
		imageResource, imgResErr := http.Get(artworkUrl)
		if imgResErr != nil {
			warnLogger.Println("Failed to load value image resource for game " + game.Name + "URL: " + artworkUrl + " - " + imgResErr.Error())
			return
		}
		coverImage, _, imgErr := image.Decode(imageResource.Body)
		if imgErr != nil {
			warnLogger.Println("Failed to load value image for game " + game.Name + "URL: " + artworkUrl + " - " + imgErr.Error())
			return
		}
		canvasCoverImage := canvas.NewImageFromImage(coverImage)
		canvasCoverImage.FillMode = canvas.ImageFillContain
		backgroundTransitionsNumber, mainPropsError := strconv.Atoi(mainProps.MustGet("visualizer.image.background.transitions"))
		if mainPropsError != nil {
			errorLogger.Println("Failed to load value for visualizer.image.background.transitions. Must me numeric : " + mainPropsError.Error())
			return
		}
		imageCoverTime, mainPropsError := strconv.Atoi(mainProps.MustGet("visualizer.image.time.seconds"))
		if mainPropsError != nil {
			errorLogger.Println("Failed to load value for visualizer.image.time.seconds. Must me numeric : " + mainPropsError.Error())
			return
		}
		sleepDuration := time.Millisecond * time.Duration(1000*(imageCoverTime/backgroundTransitionsNumber))
		for i := 0; i <= backgroundTransitionsNumber; i++ {
			showBackgroundGame(game, visualizerWindow, canvasCoverImage, sleepDuration)
		}
	}
}

func showBackgroundGame(game domain.ClientGame, visualizerWindow fyne.Window, canvasCoverImage *canvas.Image, sleepDuration time.Duration) {
	windowLayout := layout.NewMaxLayout()
	if len(game.Artworks) > 1 {
		artworkUrl := "https://images.igdb.com/igdb/image/upload/t_original/" + game.Artworks[rand.Intn(len(game.Artworks)-1)+1].ArtworkId + ".jpg"
		backgroundResource, imgResErr := http.Get(artworkUrl)
		if imgResErr != nil {
			warnLogger.Println("Failed to load value image resource for game " + game.Name + "URL: " + artworkUrl + " - " + imgResErr.Error())
			return
		}
		backgroundImage, _, imgErr := image.Decode(backgroundResource.Body)
		if imgErr != nil {
			warnLogger.Println("Failed to load value image for game " + game.Name + "URL: " + artworkUrl + " - " + imgErr.Error())
			return
		}
		blurredBackgroundImage, blurErr := stackblur.Run(backgroundImage, 20)
		if blurErr != nil {
			warnLogger.Println("Failed to blur image for game " + game.Name + "URL: " + artworkUrl + " - " + blurErr.Error())
			return
		}
		canvasBackground := canvas.NewImageFromImage(blurredBackgroundImage)
		content := container.New(windowLayout, canvasBackground, canvasCoverImage)
		visualizerWindow.SetContent(content)
	} else {
		content := container.New(windowLayout, canvasCoverImage)
		visualizerWindow.SetContent(content)
	}
	time.Sleep(sleepDuration)
}
