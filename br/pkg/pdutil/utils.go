// Copyright 2020 PingCAP, Inc. Licensed under Apache-2.0.

package pdutil

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pingcap/errors"
	berrors "github.com/tikv/migration/br/pkg/errors"
	"github.com/tikv/migration/br/pkg/httputil"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/tikv/pd/pkg/codec"
	"github.com/tikv/pd/server/schedule/placement"
)

// UndoFunc is a 'undo' operation of some undoable command.
// (e.g. RemoveSchedulers).
type UndoFunc func(context.Context) error

// Nop is the 'zero value' of undo func.
var Nop UndoFunc = func(context.Context) error { return nil }

const (
	resetTSURL       = "/pd/api/v1/admin/reset-ts"
	placementRuleURL = "/pd/api/v1/config/rules"
)

// ResetTS resets the timestamp of PD to a bigger value.
func ResetTS(ctx context.Context, pdAddr string, ts uint64, tlsConf *tls.Config) error {
	payload, err := json.Marshal(struct {
		TSO string `json:"tso,omitempty"`
	}{TSO: fmt.Sprintf("%d", ts)})
	if err != nil {
		return errors.Trace(err)
	}
	cli := httputil.NewClient(tlsConf)
	prefix := "http://"
	if tlsConf != nil {
		prefix = "https://"
	}
	reqURL := prefix + pdAddr + resetTSURL
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(payload)))
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return errors.Annotatef(berrors.ErrPDInvalidResponse, "pd resets TS failed: req=%v, resp=%v, err=%v", string(payload), buf.String(), err)
	}
	return nil
}

// GetPlacementRules return the current placement rules.
func GetPlacementRules(ctx context.Context, pdAddr string, tlsConf *tls.Config) ([]placement.Rule, error) {
	cli := httputil.NewClient(tlsConf)
	prefix := "http://"
	if tlsConf != nil {
		prefix = "https://"
	}
	reqURL := prefix + pdAddr + placementRuleURL
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if resp.StatusCode == http.StatusPreconditionFailed {
		return []placement.Rule{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Annotatef(berrors.ErrPDInvalidResponse, "get placement rules failed: resp=%v, err=%v, code=%d", buf.String(), err, resp.StatusCode)
	}
	var rules []placement.Rule
	err = json.Unmarshal(buf.Bytes(), &rules)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return rules, nil
}

// SearchPlacementRule returns the placement rule matched to the table or nil.
func SearchPlacementRule(tableID int64, placementRules []placement.Rule, role placement.PeerRoleType) *placement.Rule {
	for _, rule := range placementRules {
		key, err := hex.DecodeString(rule.StartKeyHex)
		if err != nil {
			continue
		}
		_, decoded, err := codec.DecodeBytes(key)
		if err != nil {
			continue
		}
		if rule.Role == role && tableID == tablecodec.DecodeTableID(decoded) {
			return &rule
		}
	}
	return nil
}
