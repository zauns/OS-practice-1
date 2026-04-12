package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// =================== CORES ANSI ===================
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"

	BgRed    = "\033[41m"
	BgGreen  = "\033[42m"
	BgYellow = "\033[43m"
	BgBlue   = "\033[44m"
	BgPurple = "\033[45m"
)

// =================== RECURSOS COMPARTILHADOS ===================
type Recurso struct {
	nome        string
	valor       int
	mutex       sync.Mutex
	dono        int
	totalAcessos int
}

// =================== ESTATÍSTICAS GLOBAIS ===================
type Stats struct {
	mu              sync.Mutex
	corridas        int // tentativas de race condition detectadas
	bloqueios       int // vezes que uma thread ficou bloqueada
	totalOperacoes  int
}

var stats Stats

// =================== LOGGER SINCRONIZADO ===================
var logMu sync.Mutex

func log(cor string, formato string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Printf(cor+"[%s] "+formato+Reset+"\n", append([]interface{}{timestamp}, args...)...)
}

func logSeparador(titulo string) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Printf("\n"+Bold+Cyan+"══════════════════════════════════════════════════\n")
	fmt.Printf("  %s\n", titulo)
	fmt.Printf("══════════════════════════════════════════════════"+Reset+"\n\n")
}

// =================== DEMONSTRAÇÃO DE RACE CONDITION ===================
func demonstrarRaceCondition() {
	logSeparador("PARTE 1: DEMONSTRAÇÃO DE RACE CONDITION (SEM MUTEX)")

	log(Yellow, "⚠  Executando 5 threads SEM exclusão mútua sobre variável compartilhada...")
	log(Yellow, "   Esperamos valores inconsistentes / perdas de atualização!\n")

	var contador int = 0
	var wg sync.WaitGroup

	nomes := []string{"T1", "T2", "T3", "T4", "T5"}
	cores := []string{Red, Green, Yellow, Blue, Purple}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				// Simula race condition: lê, pausa, escreve
				lido := contador
				time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				contador = lido + 1

				stats.mu.Lock()
				stats.corridas++
				stats.mu.Unlock()

				log(cores[id], "  %s leu=%d escreveu=%d | contador_atual=%d",
					nomes[id], lido, lido+1, contador)
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	logMu.Lock()
	fmt.Printf("\n"+BgRed+White+Bold+
		"  RESULTADO COM RACE CONDITION: contador = %d (esperado: 15)  "+Reset+"\n\n", contador)
	logMu.Unlock()

	log(Red, "❌ Perdemos %d atualizações devido a race conditions!", 15-contador)
}

// =================== THREAD PRINCIPAL ===================
func thread(
	id int,
	nome string,
	cor string,
	recurso1 *Recurso,
	recurso2 *Recurso,
	iteracoes int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for iter := 0; iter < iteracoes; iter++ {
		// Decide qual recurso acessar (alterna ou escolhe aleatoriamente)
		var recurso *Recurso
		if iter%2 == 0 {
			recurso = recurso1
		} else {
			recurso = recurso2
		}

		log(cor, "  %s [iter %d/%d] → tentando acessar %s...",
			nome, iter+1, iteracoes, recurso.nome)

		// TENTATIVA DE LOCK (pode bloquear aqui)
		inicio := time.Now()
		recurso.mutex.Lock()
		espera := time.Since(inicio)

		if espera > time.Millisecond {
			stats.mu.Lock()
			stats.bloqueios++
			stats.mu.Unlock()

			log(cor, "  %s ⏳ bloqueada por %v esperando %s",
				nome, espera.Round(time.Millisecond), recurso.nome)
		}

		// === SEÇÃO CRÍTICA ===
		recurso.dono = id
		recurso.totalAcessos++
		stats.mu.Lock()
		stats.totalOperacoes++
		stats.mu.Unlock()

		valorAntes := recurso.valor

		log(cor, Bold+
			"  %s ✅ ENTROU na seção crítica de %s | valor_antes=%d",
			nome, recurso.nome, valorAntes)

		// Simula trabalho: modifica o recurso
		time.Sleep(3 * time.Second) // Usa o recurso por 3 segundos
		recurso.valor += id * 10

		log(cor, "  %s 🔄 modificou %s: %d → %d | acesso #%d",
			nome, recurso.nome, valorAntes, recurso.valor, recurso.totalAcessos)

		// === FIM DA SEÇÃO CRÍTICA ===
		recurso.dono = 0
		recurso.mutex.Unlock()

		log(cor, "  %s 🔓 SAIU da seção crítica de %s",
			nome, recurso.nome)

		// Pausa entre acessos (thread "pensa" / realiza outras tarefas)
		pausa := time.Duration(rand.Intn(500)) * time.Millisecond
		time.Sleep(pausa)
	}

	log(cor, Bold+"  %s ✔ CONCLUÍDA após %d iterações", nome, iteracoes)
}

// =================== MONITOR (exibe estado periódico) ===================
func monitor(recurso1 *Recurso, recurso2 *Recurso, done chan bool) {
	nomesDono := map[int]string{
		0: "livre",
		1: "T1", 2: "T2", 3: "T3", 4: "T4", 5: "T5",
	}
	coresDono := map[int]string{
		0: Green, 1: Red, 2: Green, 3: Yellow, 4: Blue, 5: Purple,
	}

	for {
		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
			logMu.Lock()
			d1 := recurso1.dono
			d2 := recurso2.dono
			v1 := recurso1.valor
			v2 := recurso2.valor
			a1 := recurso1.totalAcessos
			a2 := recurso2.totalAcessos

			stats.mu.Lock()
			bl := stats.bloqueios
			ops := stats.totalOperacoes
			stats.mu.Unlock()

			cor1 := coresDono[d1]
			cor2 := coresDono[d2]

			fmt.Printf(Bold+Cyan+"\n  ┌─ MONITOR ──────────────────────────────────────┐\n"+Reset)
			fmt.Printf(Bold+Cyan+"  │ "+Reset+"%s%-10s"+Reset+Bold+Cyan+" val=%4d acessos=%d"+Reset+
				"  dono: "+cor1+"%-6s"+Reset+Bold+Cyan+"    │\n"+Reset,
				Cyan, recurso1.nome, v1, a1, nomesDono[d1])
			fmt.Printf(Bold+Cyan+"  │ "+Reset+"%s%-10s"+Reset+Bold+Cyan+" val=%4d acessos=%d"+Reset+
				"  dono: "+cor2+"%-6s"+Reset+Bold+Cyan+"    │\n"+Reset,
				Cyan, recurso2.nome, v2, a2, nomesDono[d2])
			fmt.Printf(Bold+Cyan+"  │ "+Reset+"bloqueios: %-4d  operações: %-4d               "+
				Bold+Cyan+"│\n"+Reset, bl, ops)
			fmt.Printf(Bold+Cyan+"  └────────────────────────────────────────────────┘\n\n"+Reset)
			logMu.Unlock()
		}
	}
}

// =================== MAIN ===================
func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Print("\033[H\033[2J") // limpa tela

	logMu.Lock()
	fmt.Println(Bold + Cyan + `
╔══════════════════════════════════════════════════════════╗
║     GERENCIAMENTO DE PROCESSOS — Sistemas Operacionais   ║
║     5 Threads · 2 Recursos · Exclusão Mútua (Mutex)      ║
╚══════════════════════════════════════════════════════════╝` + Reset)
	logMu.Unlock()

	// ─── PARTE 1: demonstrar race condition ───────────────────
	demonstrarRaceCondition()
	time.Sleep(1 * time.Second)

	// ─── PARTE 2: exclusão mútua correta ──────────────────────
	logSeparador("PARTE 2: EXCLUSÃO MÚTUA COM MUTEX (5 THREADS × 2 RECURSOS)")

	recurso1 := &Recurso{nome: "Buffer-A", valor: 0}
	recurso2 := &Recurso{nome: "Buffer-B", valor: 0}

	type configThread struct {
		id   int
		nome string
		cor  string
	}

	threads := []configThread{
		{1, "T1-Produtor", Red},
		{2, "T2-Consumidor", Green},
		{3, "T3-Escritor", Yellow},
		{4, "T4-Leitor", Blue},
		{5, "T5-Monitor", Purple},
	}

	log(White, "Iniciando 5 threads com permuta a cada ~3s por recurso...")
	log(White, "Cada thread realiza 4 iterações alternando entre Buffer-A e Buffer-B\n")

	done := make(chan bool)
	go monitor(recurso1, recurso2, done)

	var wg sync.WaitGroup
	for _, t := range threads {
		wg.Add(1)
		// Pequeno escalonamento para deixar logs mais legíveis
		time.Sleep(200 * time.Millisecond)
		go thread(t.id, t.nome, t.cor, recurso1, recurso2, 4, &wg)
	}

	wg.Wait()
	done <- true
	time.Sleep(200 * time.Millisecond)

	// ─── RELATÓRIO FINAL ──────────────────────────────────────
	logSeparador("RELATÓRIO FINAL")

	logMu.Lock()
	fmt.Printf(Bold+`
  Recursos ao final da execução:
    Buffer-A: valor=%d, total de acessos=%d
    Buffer-B: valor=%d, total de acessos=%d

  Estatísticas de sincronização:
    Bloqueios (threads que esperaram por mutex): %d
    Total de operações na seção crítica:         %d
    Race conditions demonstradas (Parte 1):      %d

  Conclusão:
    ✅  Com mutex, CADA recurso foi acessado por
        UMA thread por vez — exclusão mútua garantida.
    ❌  Sem mutex (Parte 1), race conditions causaram
        perda de atualizações no contador compartilhado.
`+Reset,
		recurso1.valor, recurso1.totalAcessos,
		recurso2.valor, recurso2.totalAcessos,
		stats.bloqueios,
		stats.totalOperacoes,
		stats.corridas,
	)
	logMu.Unlock()
}