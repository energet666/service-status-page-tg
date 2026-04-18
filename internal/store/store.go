package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const MaxItems = 1000

type StatusState string

const (
	StatusOK          StatusState = "ok"
	StatusMaintenance StatusState = "maintenance"
	StatusIncident    StatusState = "incident"
)

type AnnouncementKind string

const (
	AnnouncementInfo        AnnouncementKind = "info"
	AnnouncementMaintenance AnnouncementKind = "maintenance"
	AnnouncementIncident    AnnouncementKind = "incident"
	AnnouncementResolved    AnnouncementKind = "resolved"
	AnnouncementUser        AnnouncementKind = "user"
)

type Status struct {
	State     StatusState `json:"state"`
	Message   string      `json:"message"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type Announcement struct {
	ID        string           `json:"id"`
	Message   string           `json:"message"`
	Kind      AnnouncementKind `json:"kind"`
	CreatedAt time.Time        `json:"createdAt"`
	CreatedBy string           `json:"createdBy"`
}

type Report struct {
	ID             string    `json:"id"`
	Message        string    `json:"message"`
	Name           string    `json:"name,omitempty"`
	Contact        string    `json:"contact,omitempty"`
	IPHash         string    `json:"ipHash"`
	UserAgent      string    `json:"userAgent"`
	CreatedAt      time.Time `json:"createdAt"`
	SentToTelegram bool      `json:"sentToTelegram"`
}

type State struct {
	Status        Status         `json:"status"`
	Announcements []Announcement `json:"announcements"`
	Reports       []Report       `json:"reports"`
}

type Store struct {
	path        string
	mu          sync.Mutex
	data        State
	subscribers map[chan struct{}]struct{}
}

func Open(path string) (*Store, error) {
	st := &Store{path: path}
	if err := st.load(); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(s.data)
}

func (s *Store) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	s.mu.Lock()
	if s.subscribers == nil {
		s.subscribers = make(map[chan struct{}]struct{})
	}
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}

	return ch, unsubscribe
}

func (s *Store) SetStatus(state StatusState, message, createdBy string) (Announcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.data.Status = Status{State: state, Message: message, UpdatedAt: now}
	ann := Announcement{
		ID:        newID(),
		Message:   message,
		Kind:      announcementKindForStatus(state),
		CreatedAt: now,
		CreatedBy: createdBy,
	}
	s.prependAnnouncement(ann)
	if err := s.saveLocked(); err != nil {
		return ann, err
	}
	s.broadcastLocked()
	return ann, nil
}

func (s *Store) Resolve(message, createdBy string) (Announcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.data.Status = Status{State: StatusOK, Message: message, UpdatedAt: now}
	ann := Announcement{
		ID:        newID(),
		Message:   message,
		Kind:      AnnouncementResolved,
		CreatedAt: now,
		CreatedBy: createdBy,
	}
	s.prependAnnouncement(ann)
	if err := s.saveLocked(); err != nil {
		return ann, err
	}
	s.broadcastLocked()
	return ann, nil
}

func (s *Store) AddAnnouncement(message string, kind AnnouncementKind, createdBy string) (Announcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ann := Announcement{
		ID:        newID(),
		Message:   message,
		Kind:      kind,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
	}
	s.prependAnnouncement(ann)
	if err := s.saveLocked(); err != nil {
		return ann, err
	}
	s.broadcastLocked()
	return ann, nil
}

func (s *Store) AddReport(report Report) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if report.ID == "" {
		report.ID = newID()
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}
	createdBy := report.Name
	if createdBy == "" {
		createdBy = "user"
	}
	s.data.Reports = append([]Report{report}, s.data.Reports...)
	if len(s.data.Reports) > MaxItems {
		s.data.Reports = s.data.Reports[:MaxItems]
	}
	s.prependAnnouncement(Announcement{
		ID:        report.ID,
		Message:   report.Message,
		Kind:      AnnouncementUser,
		CreatedAt: report.CreatedAt,
		CreatedBy: createdBy,
	})
	if err := s.saveLocked(); err != nil {
		return report, err
	}
	s.broadcastLocked()
	return report, nil
}

func (s *Store) MarkReportSent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Reports {
		if s.data.Reports[i].ID == id {
			s.data.Reports[i].SentToTelegram = true
			if err := s.saveLocked(); err != nil {
				return err
			}
			s.broadcastLocked()
			return nil
		}
	}
	return nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = defaultState()
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("read state file: %w", err)
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return fmt.Errorf("decode state file: %w", err)
	}
	if s.data.Status.State == "" {
		s.data.Status = defaultState().Status
	}
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

func (s *Store) prependAnnouncement(ann Announcement) {
	s.data.Announcements = append([]Announcement{ann}, s.data.Announcements...)
	if len(s.data.Announcements) > MaxItems {
		s.data.Announcements = s.data.Announcements[:MaxItems]
	}
}

func (s *Store) broadcastLocked() {
	for ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func defaultState() State {
	now := time.Now().UTC()
	return State{
		Status: Status{
			State:     StatusOK,
			Message:   "Сервис работает штатно",
			UpdatedAt: now,
		},
		Announcements: []Announcement{},
		Reports:       []Report{},
	}
}

func announcementKindForStatus(state StatusState) AnnouncementKind {
	switch state {
	case StatusMaintenance:
		return AnnouncementMaintenance
	case StatusIncident:
		return AnnouncementIncident
	default:
		return AnnouncementInfo
	}
}

func cloneState(in State) State {
	out := in
	out.Announcements = append([]Announcement{}, in.Announcements...)
	out.Reports = append([]Report{}, in.Reports...)
	return out
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
