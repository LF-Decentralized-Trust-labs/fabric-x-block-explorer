# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: sqlc

sqlc: ## Generate Go code from SQL using sqlc
	@echo "Generating Go code from SQL files..."
	sqlc generate
	@echo "✅ SQLC code generation complete"
