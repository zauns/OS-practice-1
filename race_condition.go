package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
//  Recursos compartilhados (variáveis globais)
// ============================================================

var (
	recurso1 int64 = 0
	recurso2 int64 = 0
)

// Mutexes para exclusão mútua
var (
	mutex1 sync.Mutex
	mutex2 sync.Mutex
)

// Contadores para verificação de resultado esperado
var (
	iteracoesPorThread = 3                                      // cada thread acessa cada recurso 3 vezes
	numThreads         = 5                                      // 5 threads concorrentes
	esperado           = int64(numThreads * iteracoesPorThread) // 15
)

// ============================================================
//  Cores ANSI
// ============================================================

const (
	Reset    = "\033[0m"
	Negrito  = "\033[1m"
	Vermelho = "\033[31m"
	Verde    = "\033[32m"
	Amarelo  = "\033[33m"
	Azul     = "\033[34m"
	Magenta  = "\033[35m"
	Ciano    = "\033[36m"
	Branco   = "\033[37m"
)

var coresThreads = []string{Vermelho, Verde, Amarelo, Azul, Magenta}
var nomesThreads = []string{"A", "B", "C", "D", "E"}

// ============================================================
//  Mutex global para serializar prints (evita output embaralhado)
// ============================================================

var printMu sync.Mutex

func logf(cor, formato string, args ...interface{}) {
	printMu.Lock()
	fmt.Printf(cor+formato+Reset+"\n", args...)
	printMu.Unlock()
}

// ============================================================
//  Acesso SEM exclusão mútua (demonstra race condition)
// ============================================================

// acessarSemMutex faz read-modify-write em recurso compartilhado
// sem nenhum lock. Com múltiplas goroutines concorrentes, isso
// produz race condition real e detectável pelo -race detector.
func acessarSemMutex(id int, nome, cor string, recurso *int64, nomeRec string) {
	// --- Leitura ---
	valorLido := atomic.LoadInt64(recurso) // lê o valor atual (apenas para log)
	logf(cor, "[Thread %d - %s] 📖 Lendo %s: valor atual = %d", id, nome, nomeRec, valorLido)

	// Simula tempo de processamento — janela de vulnerabilidade para race condition
	// Outras goroutines podem ler o MESMO valor aqui antes de qualquer escrita.
	time.Sleep(500 * time.Millisecond)

	// --- Escrita não-atômica (race condition proposital) ---
	// Lê, incrementa e escreve sem atomicidade — operação 3 etapas.
	v := *recurso // (1) lê
	v++           // (2) incrementa
	*recurso = v  // (3) escreve  ← outra goroutine pode ter escrito entre (1) e (3)

	logf(cor, "[Thread %d - %s] ✏️  Escrevendo %s: %d -> %d", id, nome, nomeRec, valorLido, *recurso)

	time.Sleep(300 * time.Millisecond)
}

// ============================================================
//  Acesso COM exclusão mútua (mutex)
// ============================================================

func acessarComMutex(id int, nome, cor string, recurso *int64, nomeRec string, mu *sync.Mutex) {
	logf(cor, "[Thread %d - %s] 🔒 Tentando adquirir lock de %s...", id, nome, nomeRec)

	mu.Lock() // bloqueia aqui se outro detém o lock
	logf(cor, "[Thread %d - %s] ✅ Lock de %s ADQUIRIDO", id, nome, nomeRec)

	// --- Seção crítica ---
	valorAntes := *recurso
	logf(cor, "[Thread %d - %s] 📖 Lendo %s: valor = %d", id, nome, nomeRec, valorAntes)

	time.Sleep(500 * time.Millisecond) // mesmo delay da versão sem mutex

	*recurso = valorAntes + 1
	logf(cor, "[Thread %d - %s] ✏️  Escrevendo %s: %d -> %d", id, nome, nomeRec, valorAntes, *recurso)

	time.Sleep(300 * time.Millisecond)
	// --- Fim da seção crítica ---

	logf(cor, "[Thread %d - %s] 🔓 Liberando lock de %s", id, nome, nomeRec)
	mu.Unlock()
}

// ============================================================
//  Worker — executa iteracoesPorThread ciclos
// ============================================================

func worker(id int, usarMutex bool, wg *sync.WaitGroup, iniciar chan struct{}) {
	defer wg.Done()

	nome := nomesThreads[id-1]
	cor := coresThreads[id-1]

	// Todas as goroutines aguardam o sinal de largada simultâneo
	<-iniciar

	for i := 0; i < iteracoesPorThread; i++ {
		logf(cor, "[Thread %d - %s] ══ Ciclo %d/%d ══", id, nome, i+1, iteracoesPorThread)

		if usarMutex {
			acessarComMutex(id, nome, cor, &recurso1, "Recurso1", &mutex1)
			time.Sleep(50 * time.Millisecond)
			acessarComMutex(id, nome, cor, &recurso2, "Recurso2", &mutex2)
		} else {
			acessarSemMutex(id, nome, cor, &recurso1, "Recurso1")
			time.Sleep(50 * time.Millisecond)
			acessarSemMutex(id, nome, cor, &recurso2, "Recurso2")
		}

		// Pausa entre ciclos para interleaving mais visível
		time.Sleep(200 * time.Millisecond)
	}

	logf(cor, "[Thread %d - %s] 🏁 Finalizada", id, nome)
}

// ============================================================
//  Demonstração 1 — Race Condition (sem mutex)
// ============================================================

func demonstrarRaceCondition() {
	sep := strings.Repeat("═", 68)
	fmt.Println()
	fmt.Println(Vermelho + Negrito + sep)
	fmt.Println("  ⚠️  FASE 1: RACE CONDITION — SEM EXCLUSÃO MÚTUA")
	fmt.Println("  5 goroutines concorrem simultaneamente ao mesmo recurso")
	fmt.Println("  Resultado esperado para cada recurso: " + fmt.Sprint(esperado))
	fmt.Println(sep + Reset)
	fmt.Println()

	// Reseta recursos
	recurso1 = 0
	recurso2 = 0

	var wg sync.WaitGroup
	iniciar := make(chan struct{}) // canal de largada

	// Lança as 5 goroutines — todas bloqueadas esperando o canal fechar
	for i := 1; i <= numThreads; i++ {
		wg.Add(1)
		go worker(i, false, &wg, iniciar)
	}

	// Sinal de largada simultâneo para todas as goroutines
	logf(Amarelo, "🚦 LARGADA — todas as %d threads iniciando simultaneamente!", numThreads)
	close(iniciar)

	wg.Wait()

	fmt.Println()
	fmt.Println(Vermelho + Negrito + strings.Repeat("─", 68))
	fmt.Printf("  RESULTADO FINAL (sem mutex)\n")
	fmt.Printf("  Recurso1 → esperado: %d  |  obtido: %d\n", esperado, recurso1)
	fmt.Printf("  Recurso2 → esperado: %d  |  obtido: %d\n", esperado, recurso2)
	if recurso1 != esperado || recurso2 != esperado {
		fmt.Println("  ❌ RACE CONDITION DETECTADA! Valores incorretos.")
	} else {
		fmt.Println("  ⚠️  Valores acidentalmente corretos (tente novamente ou use -race).")
	}
	fmt.Println(strings.Repeat("─", 68) + Reset)
}

// ============================================================
//  Demonstração 2 — Exclusão Mútua (com mutex)
// ============================================================

func demonstrarExclusaoMutua() {
	sep := strings.Repeat("═", 68)
	fmt.Println()
	fmt.Println(Verde + Negrito + sep)
	fmt.Println("  🔐 FASE 2: EXCLUSÃO MÚTUA — COM MUTEX")
	fmt.Println("  5 goroutines concorrem simultaneamente — mutex serializa o acesso")
	fmt.Println("  Resultado esperado para cada recurso: " + fmt.Sprint(esperado))
	fmt.Println(sep + Reset)
	fmt.Println()

	// Reseta recursos
	recurso1 = 0
	recurso2 = 0

	var wg sync.WaitGroup
	iniciar := make(chan struct{})

	for i := 1; i <= numThreads; i++ {
		wg.Add(1)
		go worker(i, true, &wg, iniciar)
	}

	logf(Amarelo, "🚦 LARGADA — todas as %d threads iniciando simultaneamente!", numThreads)
	close(iniciar)

	wg.Wait()

	fmt.Println()
	fmt.Println(Verde + Negrito + strings.Repeat("─", 68))
	fmt.Printf("  RESULTADO FINAL (com mutex)\n")
	fmt.Printf("  Recurso1 → esperado: %d  |  obtido: %d\n", esperado, recurso1)
	fmt.Printf("  Recurso2 → esperado: %d  |  obtido: %d\n", esperado, recurso2)
	if recurso1 == esperado && recurso2 == esperado {
		fmt.Println("  ✅ SUCESSO! Exclusão mútua funcionando corretamente.")
	} else {
		fmt.Println("  ❌ ERRO INESPERADO! O mutex não protegeu os recursos.")
	}
	fmt.Println(strings.Repeat("─", 68) + Reset)
}

// ============================================================
//  Main
// ============================================================

func main() {
	sep := strings.Repeat("═", 68)
	fmt.Println(Ciano + Negrito + sep)
	fmt.Println("  SIMULADOR DE CONCORRÊNCIA REAL — GOROUTINES vs RECURSOS")
	fmt.Println("  5 goroutines  |  2 recursos compartilhados  |  3 ciclos cada")
	fmt.Println("  Concorrência REAL: todas as threads rodam ao mesmo tempo")
	fmt.Println()
	fmt.Println("  execute com 'go run -race race_condition.go'")
	fmt.Println("  para o race detector detectar a Fase 1 automaticamente")
	fmt.Println(sep + Reset)

	demonstrarRaceCondition()

	fmt.Println()
	fmt.Println(Ciano + "  Pressione ENTER para continuar para a Fase 2 (Exclusão Mútua)..." + Reset)
	fmt.Scanln()

	demonstrarExclusaoMutua()

	fmt.Println()
	fmt.Println(Ciano + Negrito + sep)
	fmt.Println("  SIMULAÇÃO CONCLUÍDA!")
	fmt.Println(sep + Reset)
}
