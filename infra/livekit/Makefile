.PHONY: up down logs token-server clean

up:
	@echo ">> Starting LiveKit..."
	docker compose --env-file .env up -d

down:
	@echo ">> Stopping LiveKit..."
	docker compose --env-file .env down

logs:
	docker compose --env-file .env logs -f livekit

token-server:
	@echo ">> Starting token server on :8081..."
	cd testclient/token-server && go run main.go

clean:
	@echo ">> Cleaning up LiveKit..."
	docker compose --env-file .env down -v --remove-orphans
