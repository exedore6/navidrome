package persistence

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/astaxie/beego/orm"
	"github.com/deluan/navidrome/conf"
	"github.com/deluan/navidrome/log"
	"github.com/deluan/navidrome/model"
	"github.com/deluan/navidrome/utils"
	"github.com/deluan/rest"
)

type artistRepository struct {
	sqlRepository
	sqlRestful
	indexGroups utils.IndexGroups
}

type dbArtist struct {
	model.Artist
	SimilarArtists string `json:"similarArtists"`
}

func NewArtistRepository(ctx context.Context, o orm.Ormer) model.ArtistRepository {
	r := &artistRepository{}
	r.ctx = ctx
	r.ormer = o
	r.indexGroups = utils.ParseIndexGroups(conf.Server.IndexGroups)
	r.tableName = "artist"
	r.sortMappings = map[string]string{
		"name": "order_artist_name",
	}
	r.filterMappings = map[string]filterFunc{
		"name":    fullTextFilter,
		"starred": booleanFilter,
	}
	return r
}

func (r *artistRepository) selectArtist(options ...model.QueryOptions) SelectBuilder {
	return r.newSelectWithAnnotation("artist.id", options...).Columns("*")
}

func (r *artistRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	return r.count(r.newSelectWithAnnotation("artist.id"), options...)
}

func (r *artistRepository) Exists(id string) (bool, error) {
	return r.exists(Select().Where(Eq{"id": id}))
}

func (r *artistRepository) Put(a *model.Artist) error {
	a.FullText = getFullText(a.Name, a.SortArtistName)
	dba := r.fromModel(a)
	_, err := r.put(dba.ID, dba)
	return err
}

func (r *artistRepository) Get(id string) (*model.Artist, error) {
	sel := r.selectArtist().Where(Eq{"id": id})
	var dba []dbArtist
	if err := r.queryAll(sel, &dba); err != nil {
		return nil, err
	}
	if len(dba) == 0 {
		return nil, model.ErrNotFound
	}
	res := r.toModels(dba)
	return &res[0], nil
}

func (r *artistRepository) GetAll(options ...model.QueryOptions) (model.Artists, error) {
	sel := r.selectArtist(options...)
	var dba []dbArtist
	err := r.queryAll(sel, &dba)
	res := r.toModels(dba)
	return res, err
}

func (r *artistRepository) toModels(dba []dbArtist) model.Artists {
	res := model.Artists{}
	for i := range dba {
		a := dba[i]
		res = append(res, *r.toModel(&a))
	}
	return res
}

func (r *artistRepository) toModel(dba *dbArtist) *model.Artist {
	a := dba.Artist
	a.SimilarArtists = nil
	for _, s := range strings.Split(dba.SimilarArtists, ";") {
		fields := strings.Split(s, ":")
		if len(fields) != 2 {
			continue
		}
		name, _ := url.QueryUnescape(fields[1])
		a.SimilarArtists = append(a.SimilarArtists, model.Artist{
			ID:   fields[0],
			Name: name,
		})
	}
	return &a
}

func (r *artistRepository) fromModel(a *model.Artist) *dbArtist {
	dba := &dbArtist{Artist: *a}
	var sa []string

	for _, s := range a.SimilarArtists {
		sa = append(sa, fmt.Sprintf("%s:%s", s.ID, url.QueryEscape(s.Name)))
	}

	dba.SimilarArtists = strings.Join(sa, ";")
	return dba
}

func (r *artistRepository) getIndexKey(a *model.Artist) string {
	name := strings.ToLower(utils.NoArticle(a.Name))
	for k, v := range r.indexGroups {
		key := strings.ToLower(k)
		if strings.HasPrefix(name, key) {
			return v
		}
	}
	return "#"
}

// TODO Cache the index (recalculate when there are changes to the DB)
func (r *artistRepository) GetIndex() (model.ArtistIndexes, error) {
	all, err := r.GetAll(model.QueryOptions{Sort: "order_artist_name"})
	if err != nil {
		return nil, err
	}

	fullIdx := make(map[string]*model.ArtistIndex)
	for i := range all {
		a := all[i]
		ax := r.getIndexKey(&a)
		idx, ok := fullIdx[ax]
		if !ok {
			idx = &model.ArtistIndex{ID: ax}
			fullIdx[ax] = idx
		}
		idx.Artists = append(idx.Artists, a)
	}
	var result model.ArtistIndexes
	for _, idx := range fullIdx {
		result = append(result, *idx)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (r *artistRepository) Refresh(ids ...string) error {
	chunks := utils.BreakUpStringSlice(ids, 100)
	for _, chunk := range chunks {
		err := r.refresh(chunk...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *artistRepository) refresh(ids ...string) error {
	type refreshArtist struct {
		model.Artist
		CurrentId string
	}
	var artists []refreshArtist
	sel := Select("f.album_artist_id as id", "f.album_artist as name", "count(*) as album_count", "a.id as current_id",
		"group_concat(f.mbz_album_artist_id , ' ') as mbz_artist_id",
		"f.sort_album_artist_name as sort_artist_name", "f.order_album_artist_name as order_artist_name",
		"sum(f.song_count) as song_count", "sum(f.size) as size").
		From("album f").
		LeftJoin("artist a on f.album_artist_id = a.id").
		Where(Eq{"f.album_artist_id": ids}).
		GroupBy("f.album_artist_id").OrderBy("f.id")
	err := r.queryAll(sel, &artists)
	if err != nil {
		return err
	}

	toInsert := 0
	toUpdate := 0
	for _, ar := range artists {
		if ar.CurrentId != "" {
			toUpdate++
		} else {
			toInsert++
		}
		ar.MbzArtistID = getMbzId(r.ctx, ar.MbzArtistID, r.tableName, ar.Name)
		err := r.Put(&ar.Artist)
		if err != nil {
			return err
		}
	}
	if toInsert > 0 {
		log.Debug(r.ctx, "Inserted new artists", "totalInserted", toInsert)
	}
	if toUpdate > 0 {
		log.Debug(r.ctx, "Updated artists", "totalUpdated", toUpdate)
	}
	return err
}

func (r *artistRepository) GetStarred(options ...model.QueryOptions) (model.Artists, error) {
	sq := r.selectArtist(options...).Where("starred = true")
	var dba []dbArtist
	err := r.queryAll(sq, &dba)
	starred := r.toModels(dba)
	return starred, err
}

func (r *artistRepository) purgeEmpty() error {
	del := Delete(r.tableName).Where("id not in (select distinct(album_artist_id) from album)")
	c, err := r.executeSQL(del)
	if err == nil {
		if c > 0 {
			log.Debug(r.ctx, "Purged empty artists", "totalDeleted", c)
		}
	}
	return err
}

func (r *artistRepository) Search(q string, offset int, size int) (model.Artists, error) {
	var dba []dbArtist
	err := r.doSearch(q, offset, size, &dba, "name")
	if err != nil {
		return nil, err
	}
	return r.toModels(dba), nil
}

func (r *artistRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(options...))
}

func (r *artistRepository) Read(id string) (interface{}, error) {
	return r.Get(id)
}

func (r *artistRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(options...))
}

func (r *artistRepository) EntityName() string {
	return "artist"
}

func (r *artistRepository) NewInstance() interface{} {
	return &model.Artist{}
}

func (r artistRepository) Delete(id string) error {
	return r.delete(Eq{"id": id})
}

func (r artistRepository) Save(entity interface{}) (string, error) {
	artist := entity.(*model.Artist)
	err := r.Put(artist)
	return artist.ID, err
}

func (r artistRepository) Update(entity interface{}, cols ...string) error {
	artist := entity.(*model.Artist)
	return r.Put(artist)
}

var _ model.ArtistRepository = (*artistRepository)(nil)
var _ model.ResourceRepository = (*artistRepository)(nil)
var _ rest.Persistable = (*artistRepository)(nil)
