// Package store 는 레코드를 로컬 SQLite 파일에 저장한다. 데이터는 교사 PC 를
// 떠나지 않는다. modernc.org/sqlite(순수 Go)를 써서 CGO 없이 크로스컴파일된다.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound 는 해당 id 의 레코드가 없을 때.
var ErrNotFound = errors.New("레코드를 찾을 수 없습니다")

// 컬렉션/필드 이름 화이트리스트(JSON path 주입 방지).
var nameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// 출력 객체에서 서버가 강제로 채우는 예약 키.
const (
	keyID        = "id"
	keyCreatedAt = "createdAt"
	keyUpdatedAt = "updatedAt"
)

type Store struct {
	db *sql.DB
}

// Open 은 path 의 SQLite 파일을 열고 스키마를 보장한다.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 쓰기 직렬화(WAL 이어도 단순·안전하게)
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id         TEXT PRIMARY KEY,
			collection TEXT NOT NULL,
			data       TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_records_col_created
			ON records(collection, created_at);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ListOpts 는 목록 조회 옵션이다.
type ListOpts struct {
	Sort    string            // 정렬 필드(기본 createdAt). createdAt/updatedAt/id 또는 데이터 필드.
	Desc    bool              // 내림차순
	Limit   int               // 0 이면 기본 상한
	Filters map[string]string // 데이터 필드 정확 일치 필터
}

const defaultLimit = 500
const maxLimit = 5000

// Insert 는 새 레코드를 만든다. data 는 JSON 객체여야 한다(서버가 id/createdAt 부여).
func (s *Store) Insert(collection string, data json.RawMessage) (map[string]any, error) {
	if !nameRe.MatchString(collection) {
		return nil, fmt.Errorf("잘못된 컬렉션 이름: %q", collection)
	}
	obj, err := toObject(data)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	id := newID(now)

	// 예약 키는 저장하지 않는다(출력 시 서버 값으로 덮어쓴다).
	delete(obj, keyID)
	delete(obj, keyCreatedAt)
	delete(obj, keyUpdatedAt)
	stored, _ := json.Marshal(obj)

	if _, err := s.db.Exec(
		`INSERT INTO records(id, collection, data, created_at, updated_at) VALUES(?,?,?,?,?)`,
		id, collection, string(stored), now, now,
	); err != nil {
		return nil, err
	}
	return assemble(id, now, now, stored), nil
}

// Get 은 단일 레코드를 반환한다.
func (s *Store) Get(collection, id string) (map[string]any, error) {
	var data string
	var created, updated int64
	err := s.db.QueryRow(
		`SELECT data, created_at, updated_at FROM records WHERE collection=? AND id=?`,
		collection, id,
	).Scan(&data, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return assemble(id, created, updated, []byte(data)), nil
}

// List 는 컬렉션의 레코드 목록을 반환한다.
func (s *Store) List(collection string, opts ListOpts) ([]map[string]any, error) {
	if !nameRe.MatchString(collection) {
		return nil, fmt.Errorf("잘못된 컬렉션 이름: %q", collection)
	}
	where := []string{"collection=?"}
	args := []any{collection}

	for field, val := range opts.Filters {
		if !nameRe.MatchString(field) {
			return nil, fmt.Errorf("잘못된 필터 필드: %q", field)
		}
		where = append(where, "json_extract(data, '$."+field+"')=?")
		args = append(args, val)
	}

	order := orderClause(opts.Sort, opts.Desc)

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	q := "SELECT id, data, created_at, updated_at FROM records WHERE " +
		strings.Join(where, " AND ") + order + " LIMIT " + strconv.Itoa(limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, data string
		var created, updated int64
		if err := rows.Scan(&id, &data, &created, &updated); err != nil {
			return nil, err
		}
		out = append(out, assemble(id, created, updated, []byte(data)))
	}
	return out, rows.Err()
}

// Patch 는 기존 레코드에 partial 객체를 얕게 병합한다.
func (s *Store) Patch(collection, id string, partial json.RawMessage) (map[string]any, error) {
	patch, err := toObject(partial)
	if err != nil {
		return nil, err
	}
	var data string
	var created int64
	err = s.db.QueryRow(
		`SELECT data, created_at FROM records WHERE collection=? AND id=?`,
		collection, id,
	).Scan(&data, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		obj = map[string]any{}
	}
	for k, v := range patch {
		if k == keyID || k == keyCreatedAt || k == keyUpdatedAt {
			continue // 예약 키는 수정 불가
		}
		obj[k] = v
	}
	now := time.Now().UnixMilli()
	merged, _ := json.Marshal(obj)
	if _, err := s.db.Exec(
		`UPDATE records SET data=?, updated_at=? WHERE collection=? AND id=?`,
		string(merged), now, collection, id,
	); err != nil {
		return nil, err
	}
	return assemble(id, created, now, merged), nil
}

// Delete 는 레코드를 삭제한다.
func (s *Store) Delete(collection, id string) error {
	res, err := s.db.Exec(`DELETE FROM records WHERE collection=? AND id=?`, collection, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAll 은 컬렉션의 모든 레코드를 지운다(관리 페이지 전용).
func (s *Store) DeleteAll(collection string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM records WHERE collection=?`, collection)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Count 는 컬렉션의 레코드 수를 반환한다.
func (s *Store) Count(collection string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM records WHERE collection=?`, collection).Scan(&n)
	return n, err
}

// ── 내부 헬퍼 ─────────────────────────────────────────────

func orderClause(sortField string, desc bool) string {
	dir := " ASC"
	if desc {
		dir = " DESC"
	}
	switch sortField {
	case "", keyCreatedAt:
		return " ORDER BY created_at" + dir
	case keyUpdatedAt:
		return " ORDER BY updated_at" + dir
	case keyID:
		return " ORDER BY id" + dir
	default:
		if nameRe.MatchString(sortField) {
			return " ORDER BY json_extract(data, '$." + sortField + "')" + dir
		}
		return " ORDER BY created_at" + dir
	}
}

// toObject 는 입력 JSON 을 객체(map)로 강제한다. 객체가 아니면 오류.
func toObject(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return nil, errors.New("본문은 JSON 객체여야 합니다(예: {\"name\":\"홍길동\"})")
	}
	return obj, nil
}

// assemble 은 저장된 data 에 서버 예약 필드를 얹어 출력 객체를 만든다.
func assemble(id string, created, updated int64, data []byte) map[string]any {
	obj := map[string]any{}
	if len(data) > 0 {
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.UseNumber()
		_ = dec.Decode(&obj)
	}
	obj[keyID] = id
	obj[keyCreatedAt] = created
	obj[keyUpdatedAt] = updated
	return obj
}

// newID 는 시간순 정렬성이 있는 짧은 고유 id 를 만든다(base36 시간 + 랜덤 hex).
func newID(nowMillis int64) string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return strconv.FormatInt(nowMillis, 36) + hex.EncodeToString(b[:])
}
