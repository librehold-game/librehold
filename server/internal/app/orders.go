package app

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"regexp"
	"slices"

	"librehold/internal/game"
)

// orderSpec decodes and checks an order's body, and returns the domain call to run in the world.
type orderSpec func(body json.RawMessage) (func(w WorldTx, me game.PlayerID) error, error)

type validator interface{ validate() error }

// MaxOrderIDs is the longest list of villages, reports or commands one order may name; each costs a lookup while the
// world is locked.
const MaxOrderIDs = 1000

// maxRecruitCount is more than any village can house; a larger count is refused before any game rule runs, as the
// noble coin cost is summed one unit at a time.
const maxRecruitCount = 100_000

func checkIDs(field string, ids []string) error {
	if len(ids) > MaxOrderIDs {
		return &ErrValidation{Field: field, Code: "tooMany"}
	}
	return nil
}

// savedID is the shape of the id a client gives a template or group it saves.
var savedID = regexp.MustCompile(`^[A-Za-z0-9_-]{0,32}$`)

func checkSavedID(id string) error {
	if !savedID.MatchString(id) {
		return &ErrValidation{Field: "id", Code: "invalid"}
	}
	return nil
}

func checkSendType(t string) error {
	if t != "attack" && t != "support" {
		return &ErrValidation{Field: "type", Code: "invalid"}
	}
	return nil
}

func decodeBody(body json.RawMessage, into any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		return &ErrValidation{Field: "body", Code: "invalid"}
	}
	return nil
}

func order[T any](run func(w WorldTx, me game.PlayerID, in T) error) orderSpec {
	return func(body json.RawMessage) (func(WorldTx, game.PlayerID) error, error) {
		var in T
		if err := decodeBody(body, &in); err != nil {
			return nil, err
		}
		if v, ok := any(&in).(validator); ok {
			if err := v.validate(); err != nil {
				return nil, err
			}
		}
		return func(w WorldTx, me game.PlayerID) error { return run(w, me, in) }, nil
	}
}

func checkUnits(u game.UnitCounts) error {
	for _, n := range u {
		if n < 0 {
			return &ErrValidation{Field: "units", Code: "negative"}
		}
	}
	return nil
}

type villageIn struct {
	VillageID string `json:"villageId"`
}

type buildIn struct {
	VillageID string          `json:"villageId"`
	Building  game.BuildingID `json:"building"`
}

type jobIn struct {
	VillageID string `json:"villageId"`
	JobID     string `json:"jobId"`
}

type recruitIn struct {
	VillageID string      `json:"villageId"`
	Unit      game.UnitID `json:"unit"`
	Count     int         `json:"count"`
}

func (in *recruitIn) validate() error {
	switch {
	case in.Count < 0:
		return &ErrValidation{Field: "count", Code: "negative"}
	case in.Count > maxRecruitCount:
		return &ErrValidation{Field: "count", Code: "countTooLarge"}
	}
	return nil
}

type researchIn struct {
	VillageID string      `json:"villageId"`
	Unit      game.UnitID `json:"unit"`
}

type sendIn struct {
	OriginID       string           `json:"originId"`
	X              int              `json:"x"`
	Y              int              `json:"y"`
	Units          game.UnitCounts  `json:"units"`
	Type           string           `json:"type"`
	CatapultTarget *game.BuildingID `json:"catapultTarget,omitempty"`
}

func (in *sendIn) validate() error {
	if err := checkSendType(in.Type); err != nil {
		return err
	}
	return checkUnits(in.Units)
}

type commandIn struct {
	CommandID string `json:"commandId"`
}

type interceptIn struct {
	OriginID  string          `json:"originId"`
	CommandID string          `json:"commandId"`
	Units     game.UnitCounts `json:"units"`
	Scout     bool            `json:"scout,omitempty"`
}

type withdrawIn struct {
	HostID string           `json:"hostId"`
	HomeID string           `json:"homeId"`
	Units  *game.UnitCounts `json:"units,omitempty"`
}

func (in *withdrawIn) validate() error {
	if in.Units == nil {
		return nil
	}
	return checkUnits(*in.Units)
}

type scheduleIn game.ScheduleSendInput

func (in *scheduleIn) validate() error {
	if err := checkSendType(in.Type); err != nil {
		return err
	}
	return checkUnits(in.Units)
}

type idIn struct {
	ID string `json:"id"`
}

type snipeIn struct {
	CommandIDs    []string        `json:"commandIds"`
	FromVillageID string          `json:"fromVillageId"`
	Units         game.UnitCounts `json:"units"`
}

func (in *snipeIn) validate() error {
	if err := checkIDs("commandIds", in.CommandIDs); err != nil {
		return err
	}
	return checkUnits(in.Units)
}

type tradeIn struct {
	OriginID string         `json:"originId"`
	TargetID string         `json:"targetId"`
	Amount   game.Resources `json:"amount"`
}

func (in *tradeIn) validate() error {
	if in.Amount.Wood < 0 || in.Amount.Clay < 0 || in.Amount.Iron < 0 {
		return &ErrValidation{Field: "amount", Code: "negative"}
	}
	return nil
}

type buildPlanIn struct {
	VillageID string            `json:"villageId"`
	Steps     []game.BuildingID `json:"steps"`
}

type templateIn struct {
	VillageID  string `json:"villageId"`
	TemplateID string `json:"templateId"`
}

type farmTemplateIn struct {
	Key   string          `json:"key"`
	Units game.UnitCounts `json:"units"`
}

func (in *farmTemplateIn) validate() error { return checkUnits(in.Units) }

type farmSendIn struct {
	OriginID string `json:"originId"`
	Key      string `json:"key"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

type autoFarmIn struct {
	Enabled    bool     `json:"enabled"`
	VillageIDs []string `json:"villageIds"`
}

func (in *autoFarmIn) validate() error { return checkIDs("villageIds", in.VillageIDs) }

type massBuildIn struct {
	VillageIDs []string        `json:"villageIds"`
	Building   game.BuildingID `json:"building"`
}

func (in *massBuildIn) validate() error { return checkIDs("villageIds", in.VillageIDs) }

type massRecruitIn struct {
	VillageIDs []string    `json:"villageIds"`
	Unit       game.UnitID `json:"unit"`
	UpTo       int         `json:"upTo"`
}

func (in *massRecruitIn) validate() error {
	switch {
	case in.UpTo < 0:
		return &ErrValidation{Field: "upTo", Code: "negative"}
	case in.UpTo > maxRecruitCount:
		return &ErrValidation{Field: "upTo", Code: "countTooLarge"}
	}
	return checkIDs("villageIds", in.VillageIDs)
}

type renameIn struct {
	VillageID string `json:"villageId"`
	Name      string `json:"name"`
}

type readIn struct {
	IDs  []string `json:"ids"`
	Read bool     `json:"read"`
}

func (in *readIn) validate() error { return checkIDs("ids", in.IDs) }

type groupIn game.VillageGroup

func (in *groupIn) validate() error {
	if err := checkSavedID(in.ID); err != nil {
		return err
	}
	return checkIDs("villageIds", in.VillageIDs)
}

type savedTemplateIn game.BuildTemplate

func (in *savedTemplateIn) validate() error { return checkSavedID(in.ID) }

type claimIn struct {
	QuestID   string `json:"questId"`
	VillageID string `json:"villageId"`
}

type none struct{}

var orders = map[string]orderSpec{
	"build": order(func(w WorldTx, me game.PlayerID, in buildIn) error {
		_, err := game.EnqueueBuild(w, me, in.VillageID, in.Building)
		return err
	}),
	"demolish": order(func(w WorldTx, me game.PlayerID, in buildIn) error {
		_, err := game.EnqueueDemolish(w, me, in.VillageID, in.Building)
		return err
	}),
	"cancel-build": order(func(w WorldTx, me game.PlayerID, in jobIn) error {
		_, err := game.CancelBuild(w, me, in.VillageID, in.JobID)
		return err
	}),
	"recruit": order(func(w WorldTx, me game.PlayerID, in recruitIn) error {
		_, err := game.EnqueueRecruit(w, me, in.VillageID, in.Unit, in.Count)
		return err
	}),
	"cancel-recruit": order(func(w WorldTx, me game.PlayerID, in jobIn) error {
		_, err := game.CancelRecruit(w, me, in.VillageID, in.JobID)
		return err
	}),
	"research": order(func(w WorldTx, me game.PlayerID, in researchIn) error {
		_, err := game.EnqueueResearch(w, me, in.VillageID, in.Unit)
		return err
	}),
	"cancel-research": order(func(w WorldTx, me game.PlayerID, in jobIn) error {
		_, err := game.CancelResearch(w, me, in.VillageID, in.JobID)
		return err
	}),
	"send": order(func(w WorldTx, me game.PlayerID, in sendIn) error {
		_, err := game.SendCommand(w, me, in.OriginID, in.X, in.Y, in.Units, in.Type, game.SendOpts{CatapultTarget: in.CatapultTarget})
		return err
	}),
	"intercept": order(func(w WorldTx, me game.PlayerID, in interceptIn) error {
		_, err := game.SendIntercept(w, me, in.OriginID, in.CommandID, in.Units, in.Scout)
		return err
	}),
	"cancel-command": order(func(w WorldTx, me game.PlayerID, in commandIn) error {
		return game.CancelCommand(w, me, in.CommandID)
	}),
	"withdraw-support": order(func(w WorldTx, me game.PlayerID, in withdrawIn) error {
		_, err := game.WithdrawSupport(w, me, in.HostID, in.HomeID, in.Units)
		return err
	}),
	"schedule-send": order(func(w WorldTx, me game.PlayerID, in scheduleIn) error {
		_, err := game.ScheduleSend(w, me, game.ScheduleSendInput(in))
		return err
	}),
	"cancel-scheduled": order(func(w WorldTx, me game.PlayerID, in idIn) error {
		return game.CancelScheduled(w, me, in.ID)
	}),
	"dodge": order(func(w WorldTx, me game.PlayerID, in commandIn) error {
		_, err := game.PlanDodge(w, me, in.CommandID)
		return err
	}),
	"snipe": order(func(w WorldTx, me game.PlayerID, in snipeIn) error {
		_, err := game.PlanSnipe(w, me, in.CommandIDs, in.FromVillageID, in.Units)
		return err
	}),
	"mint": order(func(w WorldTx, me game.PlayerID, in villageIn) error {
		return game.MintCoin(w, me, in.VillageID)
	}),
	"trade": order(func(w WorldTx, me game.PlayerID, in tradeIn) error {
		_, err := game.SendTrade(w, me, in.OriginID, in.TargetID, in.Amount)
		return err
	}),
	"accept-trader": order(func(w WorldTx, me game.PlayerID, in villageIn) error {
		return game.AcceptTrader(w, me, in.VillageID)
	}),
	"set-build-plan": order(func(w WorldTx, me game.PlayerID, in buildPlanIn) error {
		return game.SetBuildPlan(w, me, in.VillageID, in.Steps)
	}),
	"clear-build-plan": order(func(w WorldTx, me game.PlayerID, in villageIn) error {
		return game.ClearBuildPlan(w, me, in.VillageID)
	}),
	"apply-build-template": order(func(w WorldTx, me game.PlayerID, in templateIn) error {
		return game.ApplyBuildTemplate(w, me, in.VillageID, in.TemplateID)
	}),
	"save-build-template": order(func(w WorldTx, me game.PlayerID, in savedTemplateIn) error {
		return game.SaveBuildTemplate(w, me, game.BuildTemplate(in))
	}),
	"delete-build-template": order(func(w WorldTx, me game.PlayerID, in idIn) error {
		return game.DeleteBuildTemplate(w, me, in.ID)
	}),
	"set-farm-template": order(func(w WorldTx, me game.PlayerID, in farmTemplateIn) error {
		return game.SetFarmTemplate(w, me, in.Key, in.Units)
	}),
	"send-farm-template": order(func(w WorldTx, me game.PlayerID, in farmSendIn) error {
		_, err := game.SendFarmTemplate(w, me, in.OriginID, in.Key, in.X, in.Y)
		return err
	}),
	"send-template-c": order(func(w WorldTx, me game.PlayerID, in farmSendIn) error {
		_, err := game.SendTemplateC(w, me, in.OriginID, in.X, in.Y)
		return err
	}),
	"set-farm-filters": order(func(w WorldTx, me game.PlayerID, in game.FarmFiltersPatch) error {
		return game.SetFarmFilters(w, me, in)
	}),
	"set-auto-farm": order(func(w WorldTx, me game.PlayerID, in autoFarmIn) error {
		return game.SetAutoFarm(w, me, in.Enabled, in.VillageIDs)
	}),
	"save-group": order(func(w WorldTx, me game.PlayerID, in groupIn) error {
		return game.SaveGroup(w, me, game.VillageGroup(in))
	}),
	"delete-group": order(func(w WorldTx, me game.PlayerID, in idIn) error {
		return game.DeleteGroup(w, me, in.ID)
	}),
	"mass-build": order(func(w WorldTx, me game.PlayerID, in massBuildIn) error {
		game.MassBuild(w, me, in.VillageIDs, in.Building)
		return nil
	}),
	"mass-recruit": order(func(w WorldTx, me game.PlayerID, in massRecruitIn) error {
		game.MassRecruit(w, me, in.VillageIDs, in.Unit, in.UpTo)
		return nil
	}),
	"rename-village": order(func(w WorldTx, me game.PlayerID, in renameIn) error {
		return game.RenameVillage(w, me, in.VillageID, in.Name)
	}),
	"mark-reports-read": order(func(w WorldTx, me game.PlayerID, in readIn) error {
		return game.MarkReportsRead(w, me, in.IDs, in.Read)
	}),
	"mark-all-reports-read": order(func(w WorldTx, me game.PlayerID, _ none) error {
		return game.MarkAllReportsRead(w, me)
	}),
	"delete-report": order(func(w WorldTx, me game.PlayerID, in idIn) error {
		return game.DeleteReport(w, me, in.ID)
	}),
	"delete-read-reports": order(func(w WorldTx, me game.PlayerID, _ none) error {
		return game.DeleteReadReports(w, me)
	}),
	"claim-quest": order(func(w WorldTx, me game.PlayerID, in claimIn) error {
		_, err := game.ClaimQuest(w, me, in.QuestID, in.VillageID)
		return err
	}),
}

// OrderNames lists every order Order accepts, sorted.
func OrderNames() []string { return slices.Sorted(maps.Keys(orders)) }

// Order runs a player's order by name and returns their fresh state. Rule refusals come back as the domain
// errors they are; a body that does not decode, or carries negative amounts, is an ErrValidation.
func (s *Service) Order(ctx context.Context, acct *Account, worldID int64, name string, body json.RawMessage) (*PlayerState, error) {
	spec, ok := orders[name]
	if !ok {
		return nil, &ErrValidation{Field: "order", Code: "unknown"}
	}
	run, err := spec(body)
	if err != nil {
		return nil, err
	}
	if err := s.paced(ctx, acct); err != nil {
		return nil, err
	}
	logged := func(w WorldTx, me game.PlayerID) error {
		if err := run(w, me); err != nil {
			return err
		}
		w.LogAction(game.ActorOf(w.Player(me)), false, name, orderPayload(body))
		return nil
	}
	if err := s.inWorld(ctx, acct, worldID, logged); err != nil {
		s.countRejection(ctx, worldID, name, err)
		return nil, err
	}
	return s.state(ctx, acct, worldID)
}

func orderPayload(body json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(body)) == 0 {
		return json.RawMessage("{}")
	}
	return body
}
