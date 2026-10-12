#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
export PATH="$HOME/.local/bin:$PATH"

# Явный контекст защищает от запуска в другом кластере.
kubectl --context=minikube -n default apply -f k8s/01-config.yaml -f k8s/02-secret.yaml
kubectl --context=minikube -n default apply -f k8s/03-postgres.yaml
kubectl --context=minikube -n default rollout status deployment/postgres --timeout=300s

# Повторный запуск Job нужен и после обновления образа с новыми миграциями.
kubectl --context=minikube -n default delete job expenses-migrate --ignore-not-found --wait=true
kubectl --context=minikube -n default apply -f k8s/04-migrate.yaml
if ! kubectl --context=minikube -n default wait --for=condition=complete job/expenses-migrate --timeout=300s; then
    kubectl --context=minikube -n default describe job expenses-migrate
    kubectl --context=minikube -n default logs -l app=expenses-migrate --all-containers=true --tail=100
    exit 1
fi

kubectl --context=minikube -n default apply -f k8s/05-app.yaml
kubectl --context=minikube -n default rollout status deployment/expenses-app --timeout=180s
kubectl --context=minikube -n default get pods,services,pvc
