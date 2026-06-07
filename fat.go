// ╔══════════════════════════════════════════════════════════════════╗
// ║         Mini Simulador de Sistema de Arquivos — FAT              ║
// ║                                                                  ║
// ║  Mecanismo: FAT (File Allocation Table)                          ║
// ║  Hierarquia: 2 níveis (raiz "/" e subdiretórios diretos)         ║
// ║  Uso: ./fatsim [--size=<KB>] [--block=<KB>]                      ║
// ╚══════════════════════════════════════════════════════════════════╝

package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Constantes FAT
// ─────────────────────────────────────────────────────────────────────────────

const (
	FAT_FREE = 0  // bloco livre (não alocado)
	FAT_EOF  = -1 // último bloco de uma cadeia (End Of File)
)

// ─────────────────────────────────────────────────────────────────────────────
// Cores ANSI para terminal
// ─────────────────────────────────────────────────────────────────────────────

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cWhite  = "\033[37m"
	cDim    = "\033[2m"
)

// ─────────────────────────────────────────────────────────────────────────────
// Estruturas de Dados
// ─────────────────────────────────────────────────────────────────────────────

// FileEntry representa um arquivo ou entrada de diretório no sistema.
type FileEntry struct {
	Name       string
	SizeBytes  int       // tamanho lógico em bytes
	StartBlock int       // primeiro bloco da cadeia FAT
	IsDir      bool      // true = diretório, false = arquivo
	CreatedAt  time.Time // timestamp de criação
}

// Directory representa um diretório com seus arquivos.
type Directory struct {
	Name       string
	StartBlock int // bloco alocado para os metadados do diretório
	Files      map[string]*FileEntry
	CreatedAt  time.Time
}

// FileSystem é o núcleo do simulador.
//
//   FAT[i] == FAT_FREE (0)  → bloco i está livre
//   FAT[i] == FAT_EOF  (-1) → bloco i é o último da cadeia de algum arquivo
//   FAT[i] == N (>0)        → bloco i aponta para o bloco N (próximo da cadeia)
//
// BlockLabel[i] guarda um rótulo descritivo de cada bloco para exibição.
type FileSystem struct {
	TotalBytes int
	BlockSize  int
	NumBlocks  int
	FAT        []int    // tabela FAT propriamente dita
	BlockLabel []string // rótulo de exibição de cada bloco (ex: "notes.txt", "docs")
	Root       *Directory
	Dirs       map[string]*Directory // subdiretórios de 1º nível
	CurrentDir string                // "/" ou nome do subdiretório atual
}

// ─────────────────────────────────────────────────────────────────────────────
// Construtor
// ─────────────────────────────────────────────────────────────────────────────

func NewFileSystem(totalKB, blockKB int) *FileSystem {
	totalBytes := totalKB * 1024
	blockBytes := blockKB * 1024
	numBlocks := totalBytes / blockBytes

	fat := make([]int, numBlocks)
	labels := make([]string, numBlocks)

	// Bloco 0: reservado para os metadados da raiz (sempre EOF, nunca livre)
	fat[0] = FAT_EOF
	labels[0] = "ROOT"

	return &FileSystem{
		TotalBytes: totalBytes,
		BlockSize:  blockBytes,
		NumBlocks:  numBlocks,
		FAT:        fat,
		BlockLabel: labels,
		Root: &Directory{
			Name:       "/",
			StartBlock: 0,
			Files:      make(map[string]*FileEntry),
			CreatedAt:  time.Now(),
		},
		Dirs:       make(map[string]*Directory),
		CurrentDir: "/",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Utilitários internos de alocação
// ─────────────────────────────────────────────────────────────────────────────

// currentDirObj retorna o objeto *Directory do diretório em que o usuário está.
func (fs *FileSystem) currentDirObj() *Directory {
	if fs.CurrentDir == "/" {
		return fs.Root
	}
	return fs.Dirs[fs.CurrentDir]
}

// countFreeBlocks conta quantos blocos estão livres na FAT.
func (fs *FileSystem) countFreeBlocks() int {
	count := 0
	for _, v := range fs.FAT {
		if v == FAT_FREE {
			count++
		}
	}
	return count
}

// blocksNeeded calcula quantos blocos são necessários para um arquivo de sizeBytes.
// Usa teto: ceil(size / blockSize).
func (fs *FileSystem) blocksNeeded(sizeBytes int) int {
	return int(math.Ceil(float64(sizeBytes) / float64(fs.BlockSize)))
}

// findFreeBlocks encontra `needed` blocos livres na FAT (não precisam ser contíguos).
// Retorna erro se não houver blocos suficientes.
func (fs *FileSystem) findFreeBlocks(needed int) ([]int, error) {
	var blocks []int
	for i, v := range fs.FAT {
		if v == FAT_FREE {
			blocks = append(blocks, i)
			if len(blocks) == needed {
				return blocks, nil
			}
		}
	}
	return nil, fmt.Errorf(
		"espaço insuficiente: precisam de %d bloco(s), mas apenas %d livre(s)",
		needed, len(blocks),
	)
}

// chainBlocks monta a cadeia FAT para uma lista de blocos e atribui um rótulo.
// Ex: blocks = [3, 7, 11] → FAT[3]=7, FAT[7]=11, FAT[11]=EOF
func (fs *FileSystem) chainBlocks(blocks []int, label string) {
	for i := 0; i < len(blocks)-1; i++ {
		fs.FAT[blocks[i]] = blocks[i+1]
		fs.BlockLabel[blocks[i]] = label
	}
	last := blocks[len(blocks)-1]
	fs.FAT[last] = FAT_EOF
	fs.BlockLabel[last] = label
}

// freeChain percorre a cadeia FAT a partir de startBlock e marca todos como livres.
// Retorna os blocos que foram liberados.
func (fs *FileSystem) freeChain(startBlock int) []int {
	var freed []int
	cur := startBlock
	for cur != FAT_EOF && cur != FAT_FREE && cur >= 0 && cur < fs.NumBlocks {
		next := fs.FAT[cur]
		fs.FAT[cur] = FAT_FREE
		fs.BlockLabel[cur] = ""
		freed = append(freed, cur)
		cur = next
	}
	return freed
}

// getChain percorre e retorna todos os blocos de uma cadeia a partir de startBlock.
func (fs *FileSystem) getChain(startBlock int) []int {
	var chain []int
	cur := startBlock
	for cur != FAT_EOF && cur != FAT_FREE && cur >= 0 && cur < fs.NumBlocks {
		chain = append(chain, cur)
		cur = fs.FAT[cur]
	}
	return chain
}

// allFiles retorna todos os *FileEntry de arquivos reais (não-diretórios)
// presentes em qualquer lugar do sistema de arquivos.
func (fs *FileSystem) allFiles() []*FileEntry {
	var files []*FileEntry
	for _, f := range fs.Root.Files {
		if !f.IsDir {
			files = append(files, f)
		}
	}
	for _, d := range fs.Dirs {
		for _, f := range d.Files {
			files = append(files, f)
		}
	}
	return files
}

// ─────────────────────────────────────────────────────────────────────────────
// Comandos do simulador
// ─────────────────────────────────────────────────────────────────────────────

// CmdMkdir cria um novo subdiretório na raiz.
func (fs *FileSystem) CmdMkdir(name string) error {
	// Nomes duplicados não são permitidos
	if _, ok := fs.Root.Files[name]; ok {
		return fmt.Errorf("já existe uma entrada chamada '%s' na raiz", name)
	}
	if _, ok := fs.Dirs[name]; ok {
		return fmt.Errorf("diretório '%s' já existe", name)
	}

	// Um diretório ocupa exatamente 1 bloco (para seus metadados)
	blocks, err := fs.findFreeBlocks(1)
	if err != nil {
		return err
	}

	fs.chainBlocks(blocks, name)

	// Cria o objeto Directory
	dir := &Directory{
		Name:       name,
		StartBlock: blocks[0],
		Files:      make(map[string]*FileEntry),
		CreatedAt:  time.Now(),
	}
	fs.Dirs[name] = dir

	// Registra uma entrada na raiz marcando como diretório
	fs.Root.Files[name] = &FileEntry{
		Name:       name,
		SizeBytes:  fs.BlockSize, // diretório = 1 bloco
		StartBlock: blocks[0],
		IsDir:      true,
		CreatedAt:  time.Now(),
	}

	fmt.Printf("%s✓ Diretório '%s' criado com sucesso%s\n", cGreen, name, cReset)
	fmt.Printf("  → Bloco alocado na FAT: %s[%d]%s (FAT[%d] = EOF)\n",
		cCyan, blocks[0], cReset, blocks[0])
	return nil
}

// CmdRmdir remove um subdiretório e todos os seus arquivos.
func (fs *FileSystem) CmdRmdir(name string) error {
	dir, ok := fs.Dirs[name]
	if !ok {
		return fmt.Errorf("diretório '%s' não existe", name)
	}

	// Libera blocos de cada arquivo dentro do diretório
	for _, f := range dir.Files {
		freed := fs.freeChain(f.StartBlock)
		fmt.Printf("  → Arquivo '%s' removido | blocos liberados: %v\n", f.Name, freed)
	}

	// Libera o bloco do próprio diretório
	freed := fs.freeChain(dir.StartBlock)
	fmt.Printf("  → Bloco do diretório '%s' liberado: %v\n", name, freed)

	delete(fs.Dirs, name)
	delete(fs.Root.Files, name)

	// Se estávamos dentro desse diretório, voltamos à raiz
	if fs.CurrentDir == name {
		fs.CurrentDir = "/"
	}

	fmt.Printf("%s✓ Diretório '%s' e todo seu conteúdo foram removidos%s\n", cGreen, name, cReset)
	return nil
}

// CmdCreate cria um arquivo no diretório atual com o tamanho informado.
func (fs *FileSystem) CmdCreate(name string, sizeBytes int) error {
	if sizeBytes <= 0 {
		return fmt.Errorf("tamanho deve ser maior que zero")
	}

	dir := fs.currentDirObj()

	// Verifica duplicata no diretório atual
	if _, ok := dir.Files[name]; ok {
		return fmt.Errorf("'%s' já existe no diretório atual", name)
	}
	// Na raiz, nome não pode conflitar com subdiretório
	if fs.CurrentDir == "/" {
		if _, ok := fs.Dirs[name]; ok {
			return fmt.Errorf("já existe um diretório chamado '%s'", name)
		}
	}

	needed := fs.blocksNeeded(sizeBytes)
	blocks, err := fs.findFreeBlocks(needed)
	if err != nil {
		return err
	}

	// Rótulo para a FAT/mapa: inclui o diretório pai se não for raiz
	label := name
	if fs.CurrentDir != "/" {
		label = fs.CurrentDir + "/" + name
	}
	fs.chainBlocks(blocks, label)

	dir.Files[name] = &FileEntry{
		Name:       name,
		SizeBytes:  sizeBytes,
		StartBlock: blocks[0],
		IsDir:      false,
		CreatedAt:  time.Now(),
	}

	// Cálculo de fragmentação interna
	allocatedBytes := needed * fs.BlockSize
	internalFrag := allocatedBytes - sizeBytes

	fmt.Printf("%s✓ Arquivo '%s' criado%s\n", cGreen, name, cReset)
	fmt.Printf("  → Tamanho lógico:  %s\n", formatBytes(sizeBytes))
	fmt.Printf("  → Blocos alocados: %v (%d bloco(s), %s alocados)\n",
		blocks, needed, formatBytes(allocatedBytes))

	// Mostra a cadeia FAT montada
	if needed > 1 {
		fmt.Printf("  → Cadeia FAT:      ")
		for i, b := range blocks {
			if i < len(blocks)-1 {
				fmt.Printf("FAT[%d]=%d → ", b, blocks[i+1])
			} else {
				fmt.Printf("FAT[%d]=EOF", b)
			}
		}
		fmt.Println()
	} else {
		fmt.Printf("  → Cadeia FAT:      FAT[%d]=EOF\n", blocks[0])
	}

	// Fragmentação interna
	if internalFrag > 0 {
		fmt.Printf("  %s⚠  Fragmentação interna: %s desperdiçados no último bloco [%d]%s\n",
			cYellow, formatBytes(internalFrag), blocks[len(blocks)-1], cReset)
	} else {
		fmt.Printf("  %s✓  Sem fragmentação interna (arquivo preenche blocos exatamente)%s\n",
			cGreen, cReset)
	}
	return nil
}

// CmdDelete remove um arquivo do diretório atual.
func (fs *FileSystem) CmdDelete(name string) error {
	dir := fs.currentDirObj()

	entry, ok := dir.Files[name]
	if !ok {
		return fmt.Errorf("'%s' não encontrado no diretório atual", name)
	}
	if entry.IsDir {
		return fmt.Errorf("'%s' é um diretório; use: rmdir %s", name, name)
	}

	chain := fs.getChain(entry.StartBlock)
	fs.freeChain(entry.StartBlock)
	delete(dir.Files, name)

	fmt.Printf("%s✓ Arquivo '%s' removido%s\n", cGreen, name, cReset)
	fmt.Printf("  → Blocos liberados na FAT: %v\n", chain)
	return nil
}

// CmdLs lista os arquivos do diretório atual ou de um diretório especificado.
func (fs *FileSystem) CmdLs(target string) {
	var dir *Directory
	switch {
	case target == "" || target == ".":
		dir = fs.currentDirObj()
	case target == "/":
		dir = fs.Root
	default:
		d, ok := fs.Dirs[target]
		if !ok {
			fmt.Printf("%sErro: diretório '%s' não encontrado%s\n", cRed, target, cReset)
			return
		}
		dir = d
	}

	path := "/"
	if dir.Name != "/" {
		path = "/" + dir.Name
	}

	fmt.Printf("\n%s%s📁 %s%s\n", cBold, cCyan, path, cReset)
	fmt.Printf("  %-24s %-14s %-20s %s\n",
		"Nome", "Tamanho", "Blocos (cadeia FAT)", "Criado em")
	fmt.Printf("  %s\n", strings.Repeat("─", 72))

	if len(dir.Files) == 0 {
		fmt.Printf("  %s(diretório vazio)%s\n", cDim, cReset)
	} else {
		// Ordena nomes para exibição consistente
		names := make([]string, 0, len(dir.Files))
		for n := range dir.Files {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, n := range names {
			f := dir.Files[n]
			icon := "📄"
			if f.IsDir {
				icon = "📁"
			}
			chain := fs.getChain(f.StartBlock)
			fmt.Printf("  %s %-22s %-14s %-20v %s\n",
				icon, f.Name,
				formatBytes(f.SizeBytes),
				chain,
				f.CreatedAt.Format("15:04:05"))
		}
	}
	fmt.Println()
}

// CmdCd muda o diretório atual.
func (fs *FileSystem) CmdCd(target string) error {
	if target == "/" || target == ".." {
		fs.CurrentDir = "/"
		fmt.Printf("→ Diretório atual: %s/%s\n", cBold, cReset)
		return nil
	}
	if _, ok := fs.Dirs[target]; !ok {
		return fmt.Errorf("diretório '%s' não existe", target)
	}
	if fs.CurrentDir != "/" {
		return fmt.Errorf("profundidade máxima atingida (2 níveis apenas); use 'cd /' primeiro")
	}
	fs.CurrentDir = target
	fmt.Printf("→ Diretório atual: %s/%s%s\n", cBold, target, cReset)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Visualizações do nível baixo (FAT, Mapa de Blocos, Status)
// ─────────────────────────────────────────────────────────────────────────────

// ShowFAT exibe a tabela FAT completa com toda a informação de alocação.
func (fs *FileSystem) ShowFAT() {
	divider := strings.Repeat("─", 62)
	fmt.Printf("\n%s╔%s╗%s\n", cBold, strings.Repeat("═", 62), cReset)
	fmt.Printf("%s║%s  TABELA FAT — File Allocation Table%-26s║%s\n",
		cBold, cCyan, "", cReset)
	fmt.Printf("%s╠%s╣%s\n", cBold, strings.Repeat("═", 62), cReset)
	fmt.Printf("%s║ %-5s │ %-10s │ %-15s │ %-22s ║%s\n",
		cBold, "Bloco", "Valor FAT", "Significado", "Conteúdo", cReset)
	fmt.Printf("  %s\n", divider)

	for i, v := range fs.FAT {
		var valStr, meaning, color string
		switch {
		case v == FAT_FREE:
			valStr = "0 (LIVRE)"
			meaning = "bloco disponível"
			color = cDim
		case v == FAT_EOF:
			valStr = "-1 (EOF)"
			meaning = "fim de cadeia"
			color = cCyan
		default:
			valStr = fmt.Sprintf("%d", v)
			meaning = fmt.Sprintf("próximo → bloco %d", v)
			color = cBlue
		}

		label := fs.BlockLabel[i]
		if label == "" {
			label = "-"
		}
		fmt.Printf("  %s║%s %s%-5d%s │ %-10s │ %-15s │ %-22s %s║%s\n",
			cBold, cReset,
			color, i, cReset,
			valStr, meaning, label,
			cBold, cReset)
	}

	fmt.Printf("%s╚%s╝%s\n\n", cBold, strings.Repeat("═", 62), cReset)
}

// ShowMap exibe o mapa visual de blocos (alocados vs. livres).
func (fs *FileSystem) ShowMap() {
	// Define colunas conforme o número de blocos
	cols := 8
	if fs.NumBlocks > 32 {
		cols = 16
	}

	free := fs.countFreeBlocks()
	used := fs.NumBlocks - free

	fmt.Printf("\n%s═══ MAPA DE BLOCOS ══════════════════════════════════%s\n", cBold, cReset)
	fmt.Printf("  Disco: %s  │  Bloco: %s  │  %d blocos totais\n",
		formatBytes(fs.TotalBytes), formatBytes(fs.BlockSize), fs.NumBlocks)
	fmt.Println()

	for i := 0; i < fs.NumBlocks; i++ {
		if i%cols == 0 {
			fmt.Printf("  B%02d ", i)
		}

		if fs.FAT[i] == FAT_FREE {
			fmt.Printf("%s[----]%s", cGreen, cReset)
		} else {
			lbl := fs.BlockLabel[i]
			// Trunca para 4 chars para caber no mapa
			if len(lbl) > 4 {
				lbl = lbl[:4]
			}
			fmt.Printf("%s[%-4s]%s", cYellow, lbl, cReset)
		}

		if (i+1)%cols == 0 || i == fs.NumBlocks-1 {
			fmt.Println()
		}
	}

	fmt.Printf("\n  %s[----]%s = Livre (%d bloco(s), %s)    %s[████]%s = Usado (%d bloco(s), %s)\n\n",
		cGreen, cReset, free, formatBytes(free*fs.BlockSize),
		cYellow, cReset, used, formatBytes(used*fs.BlockSize))
}

// ShowStatus exibe o status completo do disco e as fragmentações.
func (fs *FileSystem) ShowStatus() {
	free := fs.countFreeBlocks()
	used := fs.NumBlocks - free

	fmt.Printf("\n%s═══ STATUS DO SISTEMA DE ARQUIVOS ══════════════════%s\n", cBold, cReset)
	fmt.Printf("  Mecanismo:     FAT (File Allocation Table)\n")
	fmt.Printf("  Disco total:   %s  (%d blocos × %s/bloco)\n",
		formatBytes(fs.TotalBytes), fs.NumBlocks, formatBytes(fs.BlockSize))
	fmt.Printf("  Espaço usado:  %s  (%d bloco(s))\n", formatBytes(used*fs.BlockSize), used)
	fmt.Printf("  Espaço livre:  %s  (%d bloco(s))\n", formatBytes(free*fs.BlockSize), free)

	// ── Fragmentação Interna ──────────────────────────────────────────────────
	// Ocorre quando o último bloco de um arquivo não é totalmente preenchido.
	// Exemplo: arquivo de 5KB em blocos de 4KB → usa 2 blocos (8KB alocados),
	//          mas 3KB do 2º bloco ficam inutilizados → 3KB de fragmentação interna.
	fmt.Printf("\n  %s▸ Fragmentação Interna%s\n", cBold, cReset)
	fmt.Printf("  %s(desperdício dentro do último bloco de cada arquivo)%s\n", cDim, cReset)

	totalWaste := 0
	hasInternal := false
	for _, f := range fs.allFiles() {
		allocated := fs.blocksNeeded(f.SizeBytes) * fs.BlockSize
		waste := allocated - f.SizeBytes
		if waste > 0 {
			hasInternal = true
			totalWaste += waste
			lastBlock := fs.getChain(f.StartBlock)
			last := lastBlock[len(lastBlock)-1]
			fmt.Printf("  %s⚠%s %-20s → alocado %s | desperdiça %s no bloco [%d]\n",
				cYellow, cReset,
				"'"+f.Name+"'",
				formatBytes(allocated),
				formatBytes(waste),
				last)
		}
	}
	if hasInternal {
		fmt.Printf("  %s  Desperdício total por fragmentação interna: %s%s\n",
			cRed, formatBytes(totalWaste), cReset)
	} else {
		fmt.Printf("  %s✓ Nenhuma fragmentação interna detectada%s\n", cGreen, cReset)
	}

	// ── Fragmentação Externa ──────────────────────────────────────────────────
	// Ocorre quando os blocos livres não são contíguos, formando "ilhas".
	// Na FAT isso não impede alocação (usa encadeamento), mas é relevante
	// para entender o conceito e para sistemas com alocação contígua.
	fmt.Printf("\n  %s▸ Fragmentação Externa%s\n", cBold, cReset)
	fmt.Printf("  %s(blocos livres dispersos — relevante para alocação contígua)%s\n", cDim, cReset)

	groups, groupSizes := analyzeFreeGroups(fs.FAT)
	fmt.Printf("  Blocos livres formam %d grupo(s) contíguo(s)\n", groups)
	if groups > 0 {
		fmt.Printf("  Tamanhos dos grupos: ")
		for i, sz := range groupSizes {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%d bloco(s)", sz)
		}
		fmt.Println()
	}

	if groups > 1 {
		fmt.Printf("  %s⚠ Fragmentação externa presente: espaço livre está fragmentado em %d ilhas%s\n",
			cYellow, groups, cReset)
		fmt.Printf("  %s  Na FAT: NÃO impede alocação (blocos encadeados).%s\n", cDim, cReset)
		fmt.Printf("  %s  Em alocação contígua: blocos maiores não caberiam.%s\n", cDim, cReset)
	} else if groups == 1 {
		fmt.Printf("  %s✓ Blocos livres são contíguos — sem fragmentação externa%s\n", cGreen, cReset)
	} else {
		fmt.Printf("  %s  Disco completamente ocupado ou vazio%s\n", cDim, cReset)
	}
	fmt.Println()
}

// ─────────────────────────────────────────────────────────────────────────────
// Funções auxiliares de análise e formatação
// ─────────────────────────────────────────────────────────────────────────────

// analyzeFreeGroups conta quantas "ilhas" de blocos livres contíguos existem
// e retorna seus tamanhos individuais.
func analyzeFreeGroups(fat []int) (int, []int) {
	var sizes []int
	count := 0
	inGroup := false
	for _, v := range fat {
		if v == FAT_FREE {
			if !inGroup {
				sizes = append(sizes, 0)
				inGroup = true
			}
			sizes[len(sizes)-1]++
			count++
		} else {
			inGroup = false
		}
	}
	_ = count
	return len(sizes), sizes
}

// parseSize converte strings como "3KB", "512B", "1MB" para bytes.
// Sem sufixo, assume KB.
func parseSize(s string) (int, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	switch {
	case strings.HasSuffix(s, "MB"):
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "MB"), 64)
		if err != nil {
			return 0, err
		}
		return int(n * 1024 * 1024), nil
	case strings.HasSuffix(s, "KB"):
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "KB"), 64)
		if err != nil {
			return 0, err
		}
		return int(n * 1024), nil
	case strings.HasSuffix(s, "B"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "B"))
		return n, err
	default:
		// Sem sufixo → interpreta como KB
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		return int(n * 1024), nil
	}
}

// formatBytes formata um número de bytes de forma legível (B, KB, MB).
func formatBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.2fMB", float64(b)/1024/1024)
	case b >= 1024:
		if b%1024 == 0 {
			return fmt.Sprintf("%dKB", b/1024)
		}
		return fmt.Sprintf("%.2fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Ajuda
// ─────────────────────────────────────────────────────────────────────────────

func printHelp() {
	fmt.Printf(`
%sComandos disponíveis:%s

  %smkdir%s  <nome>              Cria um subdiretório na raiz
  %srmdir%s  <nome>              Remove diretório e todo seu conteúdo
  %screate%s <nome> <tamanho>    Cria arquivo no diretório atual
                              ex: create foto.jpg 5KB
                              ex: create relatorio.pdf 1.5MB
                              ex: create micro.c 512B
  %sdelete%s <nome>              Remove arquivo do diretório atual
  %sls%s     [dir]               Lista arquivos (dir atual se omitido)
                              ex: ls   |  ls /  |  ls docs
  %scd%s     <dir | />           Entra no diretório (cd / volta à raiz)
  %spwd%s                        Mostra diretório atual
  %sfat%s                        Exibe a tabela FAT completa (baixo nível)
  %smap%s                        Exibe mapa visual de blocos
  %sstatus%s                     Exibe fragmentação e status do disco
  %shelp%s                       Exibe esta ajuda
  %sexit%s                       Sai do simulador

%sUnidades:%s B, KB, MB  (ex: 512B | 4KB | 1.5KB | 2MB)
%sHierarquia:%s Máximo 2 níveis: raiz (/) e subdiretórios diretos

`,
		cBold, cReset,
		cCyan, cReset, cCyan, cReset, cCyan, cReset,
		cCyan, cReset, cCyan, cReset, cCyan, cReset,
		cCyan, cReset, cCyan, cReset, cCyan, cReset,
		cCyan, cReset, cCyan, cReset, cCyan, cReset,
		cBold, cReset, cBold, cReset)
}

// ─────────────────────────────────────────────────────────────────────────────
// Main — REPL interativo
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	totalKB := flag.Int("size", 64, "Tamanho total do disco em KB")
	blockKB := flag.Int("block", 4, "Tamanho de cada bloco em KB")
	flag.Parse()

	// Validações dos parâmetros
	if *blockKB <= 0 || *totalKB <= 0 {
		fmt.Fprintln(os.Stderr, "Erro: tamanhos devem ser > 0")
		os.Exit(1)
	}
	if *totalKB < *blockKB {
		fmt.Fprintln(os.Stderr, "Erro: tamanho total deve ser ≥ tamanho do bloco")
		os.Exit(1)
	}
	if *totalKB%*blockKB != 0 {
		*totalKB = (*totalKB / *blockKB) * *blockKB
		fmt.Printf("%sAviso: tamanho ajustado para %dKB (múltiplo do bloco)%s\n\n",
			cYellow, *totalKB, cReset)
	}

	fs := NewFileSystem(*totalKB, *blockKB)

	// Banner de abertura
	fmt.Printf("%s", cBold)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║        Mini Simulador de Sistema de Arquivos             ║")
	fmt.Println("║             Mecanismo de Alocação: FAT                   ║")
	fmt.Println("╠══════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Disco total: %-8s │ Bloco: %-6s │ %3d blocos       ║\n",
		formatBytes(fs.TotalBytes), formatBytes(fs.BlockSize), fs.NumBlocks)
	fmt.Printf("║  Bloco 0 reservado para a raiz (/) — sempre alocado      ║\n")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("%s", cReset)
	fmt.Printf("  Digite %s'help'%s para ver todos os comandos.\n\n", cCyan, cReset)

	// Mostra o mapa inicial
	fs.ShowMap()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Prompt do REPL mostra o diretório atual
		if fs.CurrentDir == "/" {
			fmt.Printf("%sfs%s:%s/%s%s $ %s", cBold, cReset, cCyan, cReset, cBold, cReset)
		} else {
			fmt.Printf("%sfs%s:%s/%s%s $ %s", cBold, cReset, cCyan, fs.CurrentDir, cBold+""+cReset, cReset)
		}

		if !scanner.Scan() {
			break // EOF (Ctrl+D)
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		var err error

		switch cmd {

		case "exit", "quit", "sair":
			fmt.Println("Saindo do simulador. Até logo!")
			return

		case "help", "ajuda", "?":
			printHelp()

		case "pwd":
			if fs.CurrentDir == "/" {
				fmt.Println("/")
			} else {
				fmt.Printf("/%s\n", fs.CurrentDir)
			}

		// ── Diretórios ──────────────────────────────────────────────────────

		case "mkdir":
			if len(parts) < 2 {
				fmt.Printf("%sUso: mkdir <nome>%s\n", cRed, cReset)
				continue
			}
			err = fs.CmdMkdir(parts[1])
			if err == nil {
				fs.ShowMap()
			}

		case "rmdir":
			if len(parts) < 2 {
				fmt.Printf("%sUso: rmdir <nome>%s\n", cRed, cReset)
				continue
			}
			err = fs.CmdRmdir(parts[1])
			if err == nil {
				fs.ShowMap()
			}

		// ── Arquivos ─────────────────────────────────────────────────────────

		case "create", "touch":
			if len(parts) < 3 {
				fmt.Printf("%sUso: create <nome> <tamanho>   ex: create doc.txt 3KB%s\n", cRed, cReset)
				continue
			}
			size, parseErr := parseSize(parts[2])
			if parseErr != nil {
				fmt.Printf("%sErro: tamanho inválido '%s'%s\n", cRed, parts[2], cReset)
				continue
			}
			err = fs.CmdCreate(parts[1], size)
			if err == nil {
				fs.ShowMap()
			}

		case "delete", "rm", "del":
			if len(parts) < 2 {
				fmt.Printf("%sUso: delete <nome>%s\n", cRed, cReset)
				continue
			}
			err = fs.CmdDelete(parts[1])
			if err == nil {
				fs.ShowMap()
			}

		// ── Listagem / Navegação ─────────────────────────────────────────────

		case "ls", "dir", "list":
			target := ""
			if len(parts) > 1 {
				target = parts[1]
			}
			fs.CmdLs(target)

		case "cd":
			if len(parts) < 2 {
				fmt.Printf("%sUso: cd <dir> | cd /%s\n", cRed, cReset)
				continue
			}
			err = fs.CmdCd(parts[1])

		// ── Visualizações de baixo nível ────────────────────────────────────

		case "fat":
			fs.ShowFAT()

		case "map":
			fs.ShowMap()

		case "status":
			fs.ShowStatus()

		default:
			fmt.Printf("%sComando desconhecido: '%s'. Digite 'help' para ajuda.%s\n",
				cRed, cmd, cReset)
		}

		if err != nil {
			fmt.Printf("%sErro: %s%s\n", cRed, err, cReset)
		}
	}
}