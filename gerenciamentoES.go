// ╔══════════════════════════════════════════════════════════╗
// ║    Mini Simulador de Gerenciamento de E/S — C-SCAN       ║
// ║                                                          ║
// ║  Algoritmo: C-SCAN (Circular SCAN / Elevador Circular)   ║
// ║  Uso: go run gerenciamentoES.go                          ║
// ╚══════════════════════════════════════════════════════════╝

package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ─────────────────────────────────────────────────────────────
// Cores ANSI para terminal
// ─────────────────────────────────────────────────────────────

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cDim    = "\033[2m"
)

// ─────────────────────────────────────────────────────────────
// Estruturas de dados
// ─────────────────────────────────────────────────────────────

// CScanStep representa um único movimento da cabeça de disco.
type CScanStep struct {
	From     int    // posição inicial do movimento
	To       int    // bloco destino (pode ser requisição ou extremidade)
	Seek     int    // seek time parcial = |To - From|
	IsReturn bool   // true se é o pulo de retorno (max→min ou min→max)
	Label    string // observação extra ("extremidade", "retorno", "início")
}

// CScanResult agrega o resultado completo da execução do C-SCAN.
type CScanResult struct {
	Steps      []CScanStep
	TotalSeek  int
	Visited    []int // apenas blocos de requisições visitados, sem extremos
}

// ─────────────────────────────────────────────────────────────
// Algoritmo C-SCAN
// ─────────────────────────────────────────────────────────────

// runCSCAN executa o algoritmo C-SCAN.
//
// O cabeçote parte de `current` e segue na `direcao` escolhida:
//
//   direcao = "direita" (→ max):
//      1. Atende requisições ≥ current em ordem crescente
//      2. Vai até maxBlock (extremidade)
//      3. Pula para minBlock (retorno sem serviço)
//      4. Atende requisições < current em ordem crescente
//
//   direcao = "esquerda" (→ min):
//      1. Atende requisições ≤ current em ordem decrescente
//      2. Vai até minBlock (extremidade)
//      3. Pula para maxBlock (retorno sem serviço)
//      4. Atende requisições > current em ordem decrescente
func runCSCAN(current int, requests []int, minBlock, maxBlock int, direcao string) CScanResult {
	// Clona e ordena as requisições
	sorted := make([]int, len(requests))
	copy(sorted, requests)
	sort.Ints(sorted)

	var steps []CScanStep

	// ── Passo inicial: registra posição de partida ──
	steps = append(steps, CScanStep{From: current, To: current, Seek: 0, Label: "início"})

	pos := current

	if direcao == "direita" {
		// Separa requisições: right = ≥ pos, left = < pos
		var right, left []int
		for _, r := range sorted {
			if r >= pos {
				right = append(right, r)
			} else {
				left = append(left, r)
			}
		}

		// 1) Atende lado direito em ordem crescente
		for _, r := range right {
			seek := r - pos
			steps = append(steps, CScanStep{From: pos, To: r, Seek: seek})
			pos = r
		}

		// 2) Vai até a extremidade máxima
		if pos < maxBlock {
			seek := maxBlock - pos
			steps = append(steps, CScanStep{From: pos, To: maxBlock, Seek: seek, Label: "extremidade (max)"})
			pos = maxBlock
		}

		// 3) Pula de maxBlock para minBlock — retorno sem serviço
		seekRet := maxBlock - minBlock
		steps = append(steps, CScanStep{
			From:     pos,
			To:       minBlock,
			Seek:     seekRet,
			IsReturn: true,
			Label:    fmt.Sprintf("retorno → min (%d → %d)", maxBlock, minBlock),
		})
		pos = minBlock

		// 4) Atende lado esquerdo em ordem crescente (vindo de min)
		for _, r := range left {
			seek := r - pos
			steps = append(steps, CScanStep{From: pos, To: r, Seek: seek})
			pos = r
		}

	} else { // direcao == "esquerda"
		// Separa requisições: left = ≤ pos, right = > pos
		var left, right []int
		for _, r := range sorted {
			if r <= pos {
				left = append(left, r)
			} else {
				right = append(right, r)
			}
		}

		// Inverte left para ordem decrescente (indo para a esquerda)
		for i, j := 0, len(left)-1; i < j; i, j = i+1, j-1 {
			left[i], left[j] = left[j], left[i]
		}

		// 1) Atende lado esquerdo em ordem decrescente
		for _, r := range left {
			seek := pos - r
			steps = append(steps, CScanStep{From: pos, To: r, Seek: seek})
			pos = r
		}

		// 2) Vai até a extremidade mínima
		if pos > minBlock {
			seek := pos - minBlock
			steps = append(steps, CScanStep{From: pos, To: minBlock, Seek: seek, Label: "extremidade (min)"})
			pos = minBlock
		}

		// 3) Pula de minBlock para maxBlock — retorno sem serviço
		seekRet := maxBlock - minBlock
		steps = append(steps, CScanStep{
			From:     pos,
			To:       maxBlock,
			Seek:     seekRet,
			IsReturn: true,
			Label:    fmt.Sprintf("retorno → max (%d → %d)", minBlock, maxBlock),
		})
		pos = maxBlock

		// 4) Atende lado direito em ordem decrescente (vindo de max)
		sort.Sort(sort.Reverse(sort.IntSlice(right)))
		for _, r := range right {
			seek := pos - r
			steps = append(steps, CScanStep{From: pos, To: r, Seek: seek})
			pos = r
		}
	}

	// Calcula total seek
	total := 0
	for _, s := range steps {
		total += s.Seek
	}

	// Monta visited: blocos de requisições atendidos (exclui extremos/retorno/início)
	reqSet := make(map[int]int) // contagem de quantas vezes cada bloco foi pedido
	for _, r := range requests {
		reqSet[r]++
	}

	var visited []int
	for _, s := range steps {
		// Pula steps não-requisição (início, extremidade, retorno)
		if s.Label == "início" || s.IsReturn || s.Label == "extremidade (max)" || s.Label == "extremidade (min)" {
			continue
		}
		// Verifica se este bloco To é uma requisição real
		if cnt := reqSet[s.To]; cnt > 0 {
			visited = append(visited, s.To)
			reqSet[s.To] = cnt - 1 // consome uma ocorrência (para duplicatas)
		}
	}

	return CScanResult{Steps: steps, TotalSeek: total, Visited: visited}
}

// ─────────────────────────────────────────────────────────────
// Exibição dos resultados
// ─────────────────────────────────────────────────────────────

// printResult exibe a tabela completa de execução do C-SCAN.
func printResult(result CScanResult, minBlock, maxBlock int) {
	// Cabeçalho
	fmt.Printf("\n%s═══ RESULTADO — C-SCAN ═══════════════════════════════════%s\n", cBold, cReset)
	fmt.Printf("  %sIntervalo:%s [%d, %d]  |  ", cCyan, cReset, minBlock, maxBlock)
	// Determina a direção real pelo primeiro movimento com deslocamento
	for i := 1; i < len(result.Steps); i++ {
		s := result.Steps[i]
		if s.To > s.From {
			fmt.Printf("%sDireção:%s direita (→ max)\n", cCyan, cReset)
			break
		} else if s.To < s.From {
			fmt.Printf("%sDireção:%s esquerda (→ min)\n", cCyan, cReset)
			break
		}
	}

	// ── Tabela de movimentos ────────────────────────────────────────────
	fmt.Printf("\n  %-18s %-10s %-6s  %s\n",
		cBold+"Movimento"+cReset,
		cBold+"Seek"+cReset,
		cBold+"Bloco"+cReset,
		cBold+"Observação"+cReset)
	fmt.Printf("  %s\n", strings.Repeat("─", 60))

	for _, s := range result.Steps {
		if s.Label == "início" {
			// Linha de início
			fmt.Printf("  %-18s %-10s  %s%-6d%s  %s\n",
				"", "", cGreen, s.From, cReset, "← início")
			continue
		}

		mov := fmt.Sprintf("%d → %d", s.From, s.To)
		seek := fmt.Sprintf("%d u.t.", s.Seek)
		bloco := fmt.Sprintf("%d", s.To)

		// Estilo conforme tipo de movimento
		switch {
		case s.IsReturn:
			fmt.Printf("  %s%-18s%s %s%-10s%s %s%-6s%s  %s⚠ %s%s\n",
				cYellow, mov, cReset,
				cYellow, seek, cReset,
				cYellow, bloco, cReset,
				cYellow, s.Label, cReset)
		case s.Label != "":
			fmt.Printf("  %-18s %-10s %s%-6d%s  %s%s%s\n",
				mov, seek,
				cCyan, s.To, cReset,
				cDim, s.Label, cReset)
		default:
			fmt.Printf("  %-18s %-10s  %-6d\n",
				mov, seek, s.To)
		}
	}

	// ── Total ───────────────────────────────────────────────────────────
	fmt.Printf("  %s\n", strings.Repeat("─", 60))
	fmt.Printf("  %s%-18s %s%d u.t.%s\n", cBold, "TOTAL:", cGreen, result.TotalSeek, cReset)

	// ── Ordem de visitação (apenas requisições) ─────────────────────────
	fmt.Printf("\n  %sOrdem de blocos visitados (requisições):%s\n", cBold, cReset)
	fmt.Printf("  ")
	for i, b := range result.Visited {
		if i > 0 {
			fmt.Printf(" → ")
		}
		fmt.Printf("%s%d%s", cCyan, b, cReset)
	}
	fmt.Printf("\n\n")
}

// ─────────────────────────────────────────────────────────────
// Utilitários de entrada
// ─────────────────────────────────────────────────────────────

var scanner = bufio.NewScanner(os.Stdin)

// readLine lê uma linha do stdin e retorna trimada.
func readLine(prompt string) string {
	fmt.Printf("  %s", prompt)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// confirmada usada para "pressione Enter" no caso de valores inválidos.
func waitEnter() {
	fmt.Printf("  %sPressione Enter para continuar...%s", cDim, cReset)
	scanner.Scan()
}

// ─────────────────────────────────────────────────────────────
// Main — entrada interativa e execução
// ─────────────────────────────────────────────────────────────

func main() {
	// Banner
	fmt.Printf("%s", cBold)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║      Mini Simulador de Gerenciamento de E/S             ║")
	fmt.Println("║           Algoritmo: C-SCAN (Circular SCAN)             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("%s", cReset)

	// ── 1. Configuração do disco ────────────────────────────────────────
	fmt.Printf("\n%s▸ Configuração do Disco%s\n", cBold, cReset)

	minBlock := -1
	for minBlock < 0 {
		line := readLine("  Bloco mínimo (ex: 0): ")
		if v, err := strconv.Atoi(line); err == nil && v >= 0 {
			minBlock = v
		} else {
			fmt.Printf("  %sValor inválido. Digite um número ≥ 0.%s\n", cRed, cReset)
		}
	}

	maxBlock := -1
	for maxBlock < 0 || maxBlock <= minBlock {
		line := readLine("  Bloco máximo (ex: 199): ")
		if v, err := strconv.Atoi(line); err == nil && v > minBlock {
			maxBlock = v
		} else {
			fmt.Printf("  %sValor inválido. Digite um número > %d.%s\n", cRed, minBlock, cReset)
		}
	}

	// ── 2. Requisições ──────────────────────────────────────────────────
	fmt.Printf("\n%s▸ Requisições%s\n", cBold, cReset)

	var requests []int
	for {
		mode := readLine("  [m] Manual  |  [r] Aleatório  |  [s] Sair: ")
		mode = strings.ToLower(mode)

		switch mode {
		case "m", "manual":
			fmt.Printf("  Digite os blocos separados por espaço (ex: 98 183 37):\n")
			line := readLine("  ")
			parts := strings.Fields(line)
			for _, p := range parts {
				if v, err := strconv.Atoi(p); err == nil {
					if v >= minBlock && v <= maxBlock {
						requests = append(requests, v)
					} else {
						fmt.Printf("  %sBloco %d fora do intervalo [%d, %d], ignorado.%s\n",
							cYellow, v, minBlock, maxBlock, cReset)
					}
				}
			}
			if len(requests) == 0 {
				fmt.Printf("  %sNenhuma requisição válida informada.%s\n", cRed, cReset)
				continue
			}
		case "r", "random", "aleatório", "aleatorio":
			line := readLine("  Quantidade de requisições: ")
			qty, err := strconv.Atoi(line)
			if err != nil || qty <= 0 {
				fmt.Printf("  %sQuantidade inválida.%s\n", cRed, cReset)
				continue
			}
			// Gera requisições aleatórias no intervalo [minBlock, maxBlock]
			for i := 0; i < qty; i++ {
				r := rand.Intn(maxBlock-minBlock+1) + minBlock
				requests = append(requests, r)
			}
		case "s", "sair", "exit":
			fmt.Println("  Encerrando.")
			return
		default:
			fmt.Printf("  %sOpção inválida.%s\n", cRed, cReset)
			continue
		}
		break
	}

	// Mostra as requisições
	fmt.Printf("  %sRequisições:%s %v\n", cCyan, cReset, requests)

	// ── 3. Posição inicial da cabeça ────────────────────────────────────
	fmt.Printf("\n%s▸ Posição Inicial da Cabeça%s\n", cBold, cReset)

	posicao := -1
	for posicao < 0 || posicao < minBlock || posicao > maxBlock {
		line := readLine(fmt.Sprintf("  Posição inicial (ex: %d): ", (minBlock+maxBlock)/2))
		if v, err := strconv.Atoi(line); err == nil && v >= minBlock && v <= maxBlock {
			posicao = v
		} else {
			fmt.Printf("  %sValor inválido (intervalo [%d, %d]).%s\n", cRed, minBlock, maxBlock, cReset)
		}
	}

	// ── 4. Direção ──────────────────────────────────────────────────────
	fmt.Printf("\n%s▸ Direção do C-SCAN%s\n", cBold, cReset)
	direcao := ""
	for direcao == "" {
		line := readLine("  [d] Direita (→ max)  |  [e] Esquerda (→ min): ")
		line = strings.ToLower(line)
		switch line {
		case "d", "direita":
			direcao = "direita"
		case "e", "esquerda":
			direcao = "esquerda"
		default:
			fmt.Printf("  %sOpção inválida.%s\n", cRed, cReset)
		}
	}

	// ── 5. Executa C-SCAN ───────────────────────────────────────────────
	result := runCSCAN(posicao, requests, minBlock, maxBlock, direcao)

	// ── 6. Exibe resultado ──────────────────────────────────────────────
	printResult(result, minBlock, maxBlock)

	// ── 7. Pergunta se quer repetir ─────────────────────────────────────
	line := readLine("  Executar novamente com novos parâmetros? [s/N]: ")
	line = strings.ToLower(line)
	if line == "s" || line == "sim" {
		main()
	}
}
