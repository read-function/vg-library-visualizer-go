package domain

type ClientGame struct {
	Name        string            `json:"name"`
	Source      GameSource        `json:"source"`
	SourceId    string            `json:"source-id"`
	Description string            `json:"description"`
	Developers  []string          `json:"developers"`
	Artworks    []IgdbGameArtwork `json:"artworks"`
}

type GameSource int

const (
	UnknownGameSource GameSource = iota
	Steam
	ItchIo
)

func (gameSource GameSource) String() string {
	switch gameSource {
	case Steam:
		return "Steam"
	case ItchIo:
		return "itch.io"
	case UnknownGameSource:
		return "UnknownGameSource"
	}
	return "UnknownGameSource"
}

type IgdbGameArtwork struct {
	Id        int    `json:"id"`
	ArtworkId string `json:"image_id"`
}

type ArtworkType int

const (
	UnknownArtworkType ArtworkType = iota
	Artwork
	Cover
	ScreenShot
)

func (artType ArtworkType) String() string {
	switch artType {
	case Artwork:
		return "artworks"
	case Cover:
		return "covers"
	case ScreenShot:
		return "screenshots"
	case UnknownArtworkType:
		return "UnknownArtworkType"
	}
	return "UnknownArtworkType"
}
