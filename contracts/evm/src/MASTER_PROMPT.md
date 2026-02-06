# 🚀 MASTER CONTEXT: AuraProtocol (Hackathon Convergence 2026)

## 1. VISION & IDENTITÉ
* **Nom du Projet :** AuraProtocol
* **Composant Clé :** AuraValidator (Workflow pour Chainlink CRE)
* **Concept :** "RWA Guard" (Gardien d'Actifs Réels).
* **Philosophie :** Passer d'une Preuve de Réserve (PoR) passive à une **Conformité Prédictive**. Le système utilise l'IA pour détecter des risques systémiques (bank-run, dépréciation, fraude) avant la validation on-chain.

---

## 2. STACK TECHNIQUE (STRICTE)
* **Langage :** Golang 1.23+ (Code idiomatique, interfaces propres).
* **Infrastructure :** Chainlink Runtime Environment (CRE).
* **Binary :** WebAssembly (WASM).
* **SDK :** `github.com/smartcontractkit/cre-sdk-go/sdk`.
* **Capabilities :** `HTTP`, `AI`, `Consensus`.

---

## 3. ARCHITECTURE EN 3 COUCHES (STRATÉGIE "ÉTOILES")
1. **Layer 1 (The Moon) :** Fetch de données brutes via HTTP + Vérification `Reserve >= Supply`. Gestion d'erreurs stricte.
2. **Layer 2 (Robustness) :** Multi-sourcing (min. 2 sources) + Détection de déviation (Incohérence de données).
3. **Layer 3 (The Stars) :** Analyse de flux via `Capability AI` (détection de patterns de panique bancaire/retraits massifs). Le verdict final est une synthèse [Donnée + Intelligence].

---

## 4. MODE OPÉRATOIRE DE L'IA (THINKING & PLANNING)
* **Chain of Thought :** L'IA doit utiliser ses capacités de "Thinking" pour analyser chaque étape AVANT d'écrire le code.
* **Planning Mode :** Toute modification doit passer par une phase de plan détaillée. L'IA doit confirmer que la modification ne brise pas le déterminisme WASM.
* **Refactoring Constant :** Tous les 3-4 changements, l'IA doit analyser le fichier `main.go` pour s'assurer qu'il reste modulaire et lisible (Institutional Grade).

---

## 5. HALLUCINATION GUARDRAILS (SÉCURITÉ)
* **No Centralization :** Refuser toute solution qui dépend d'un serveur centralisé ou d'une base de données externe hors Capabilities CRE.
* **WASM Sandboxing :** Interdiction d'utiliser `os`, `net` (standard), ou `time.Now()` directement. Toujours passer par les abstractions du SDK Chainlink.
* **Keywords Warning :** Si l'IA commence à suggérer "API Key en clair", "Centralized DB" ou "Hardcoded Secrets", elle doit s'auto-corriger immédiatement.

---

## 6. DOCUMENTATION & RÉFÉRENCES (À COMPLÉTER LE 6 FÉV)
* **Sponsors :** [Liste à insérer le 06/02]
* **Endpoints API :** [Simulateurs institutionnels à insérer]
* **Prompt IA Interne :** "Tu es un analyste de risque financier expert. Analyse ces entrées et évalue la probabilité de défaut de réserve sur une échelle de 0 à 100."

---

## 7. MÉMOIRE DU PROJET
> "AuraProtocol vise le prix **Risk & Compliance** (12 000 $). Le code doit être digne d'une infrastructure financière mondiale. La simplicité du workflow doit cacher une robustesse extrême."