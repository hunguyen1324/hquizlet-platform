package repository

import (
	"context"
	"encoding/json"
	"fmt"
)

// AdminCredit writes the ledger adjustment and its audit entry in the same transaction.
func (r *WalletRepo) AdminCredit(ctx context.Context, actorID, userID int64, amount int, note, requestID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, err := r.InsertTransaction(ctx, tx, userID, "adjustment", "credit", amount, "", note)
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{"amountVnd": amount, "transactionId": id, "note": note})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_log(actor_id,action,resource,result,request_id,detail)
 VALUES ($1,'wallet:Credit',$2,'allow',NULLIF($3,''),$4::jsonb)`, actorID, fmt.Sprintf("hq:wallet/%d", userID), requestID, detail)
	if err != nil {
		return err
	}
	return tx.Commit()
}
