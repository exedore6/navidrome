package model

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
)

type QueryOptions struct {
	Sort    string
	Order   string
	Max     int
	Offset  int
	Filters squirrel.Sqlizer
}

type ResourceRepository interface {
	rest.Repository
}

type DataStore interface {
	Album(ctx context.Context) AlbumRepository
	Artist(ctx context.Context) ArtistRepository
	MediaFile(ctx context.Context) MediaFileRepository
	MediaFolder(ctx context.Context) MediaFolderRepository
	Genre(ctx context.Context) GenreRepository
	Playlist(ctx context.Context) PlaylistRepository
	PlayQueue(ctx context.Context) PlayQueueRepository
	Property(ctx context.Context) PropertyRepository
	User(ctx context.Context) UserRepository
	Transcoding(ctx context.Context) TranscodingRepository
	Player(ctx context.Context) PlayerRepository

	Resource(ctx context.Context, model interface{}) ResourceRepository

	WithTx(func(tx DataStore) error) error
	GC(ctx context.Context, rootFolder string) error
}
