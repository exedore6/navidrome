package scanner

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deluan/navidrome/consts"
	"github.com/deluan/navidrome/model"
	"github.com/deluan/navidrome/scanner/metadata"
	"github.com/deluan/navidrome/utils"
	"github.com/kennygrant/sanitize"
	"github.com/microcosm-cc/bluemonday"
)

type mediaFileMapper struct {
	rootFolder string
	policy     *bluemonday.Policy
}

func newMediaFileMapper(rootFolder string) *mediaFileMapper {
	return &mediaFileMapper{rootFolder: rootFolder, policy: bluemonday.UGCPolicy()}
}

func (s *mediaFileMapper) toMediaFile(md metadata.Metadata) model.MediaFile {
	mf := &model.MediaFile{}
	mf.ID = s.trackID(md)
	mf.Title = s.mapTrackTitle(md)
	mf.Album = md.Album()
	mf.AlbumID = s.albumID(md)
	mf.Album = s.mapAlbumName(md)
	mf.ArtistID = s.artistID(md)
	mf.Artist = s.mapArtistName(md)
	mf.AlbumArtistID = s.albumArtistID(md)
	mf.AlbumArtist = s.mapAlbumArtistName(md)
	mf.Genre = md.Genre()
	mf.Compilation = md.Compilation()
	mf.Year = md.Year()
	mf.TrackNumber, _ = md.TrackNumber()
	mf.DiscNumber, _ = md.DiscNumber()
	mf.DiscSubtitle = md.DiscSubtitle()
	mf.Duration = md.Duration()
	mf.BitRate = md.BitRate()
	mf.Path = md.FilePath()
	mf.Suffix = md.Suffix()
	mf.Size = md.Size()
	mf.HasCoverArt = md.HasPicture()
	mf.SortTitle = md.SortTitle()
	mf.SortAlbumName = md.SortAlbum()
	mf.SortArtistName = md.SortArtist()
	mf.SortAlbumArtistName = md.SortAlbumArtist()
	mf.OrderAlbumName = sanitizeFieldForSorting(mf.Album)
	mf.OrderArtistName = sanitizeFieldForSorting(mf.Artist)
	mf.OrderAlbumArtistName = sanitizeFieldForSorting(mf.AlbumArtist)
	mf.CatalogNum = md.CatalogNum()
	mf.MbzTrackID = md.MbzTrackID()
	mf.MbzAlbumID = md.MbzAlbumID()
	mf.MbzArtistID = md.MbzArtistID()
	mf.MbzAlbumArtistID = md.MbzAlbumArtistID()
	mf.MbzAlbumType = md.MbzAlbumType()
	mf.MbzAlbumComment = md.MbzAlbumComment()
	mf.Comment = s.policy.Sanitize(md.Comment())
	mf.Lyrics = s.policy.Sanitize(md.Lyrics())

	// TODO Get Creation time. https://github.com/djherbis/times ?
	mf.CreatedAt = md.ModificationTime()
	mf.UpdatedAt = md.ModificationTime()

	return *mf
}

func sanitizeFieldForSorting(originalValue string) string {
	v := strings.TrimSpace(sanitize.Accents(originalValue))
	return utils.NoArticle(v)
}

func (s *mediaFileMapper) mapTrackTitle(md metadata.Metadata) string {
	if md.Title() == "" {
		s := strings.TrimPrefix(md.FilePath(), s.rootFolder+string(os.PathSeparator))
		e := filepath.Ext(s)
		return strings.TrimSuffix(s, e)
	}
	return md.Title()
}

func (s *mediaFileMapper) mapAlbumArtistName(md metadata.Metadata) string {
	switch {
	case md.Compilation():
		return consts.VariousArtists
	case md.AlbumArtist() != "":
		return md.AlbumArtist()
	case md.Artist() != "":
		return md.Artist()
	default:
		return consts.UnknownArtist
	}
}

func (s *mediaFileMapper) mapArtistName(md metadata.Metadata) string {
	if md.Artist() != "" {
		return md.Artist()
	}
	return consts.UnknownArtist
}

func (s *mediaFileMapper) mapAlbumName(md metadata.Metadata) string {
	name := md.Album()
	if name == "" {
		return "[Unknown Album]"
	}
	return name
}

func (s *mediaFileMapper) trackID(md metadata.Metadata) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(md.FilePath())))
}

func (s *mediaFileMapper) albumID(md metadata.Metadata) string {
	albumPath := strings.ToLower(fmt.Sprintf("%s\\%s", s.mapAlbumArtistName(md), s.mapAlbumName(md)))
	return fmt.Sprintf("%x", md5.Sum([]byte(albumPath)))
}

func (s *mediaFileMapper) artistID(md metadata.Metadata) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(s.mapArtistName(md)))))
}

func (s *mediaFileMapper) albumArtistID(md metadata.Metadata) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(s.mapAlbumArtistName(md)))))
}
