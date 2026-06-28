//go:build windows

package agent

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
	"golang.org/x/sys/windows"
)

type debugSession struct {
	id       string
	pid      int
	handle   windows.Handle
	name     string
	path     string
	baseAddr uint64
}

type debugManager struct {
	mu       sync.Mutex
	sessions map[string]*debugSession
	nextID   int
}

func newDebugManager() *debugManager {
	return &debugManager{sessions: make(map[string]*debugSession)}
}

// handleDebugAttach attaches to a process by PID or name
func (a *Agent) handleDebugAttach(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugAttachParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	pid := params.PID
	if pid == 0 && params.ProcessName != "" {
		found, err := findProcessByName(params.ProcessName)
		if err != nil {
			return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("find process failed: %v", err))
		}
		pid = found
	}
	if pid == 0 {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, "pid or process_name required")
	}

	const PROCESS_QUERY_INFORMATION = 0x0400
	const PROCESS_VM_READ = 0x0010
	handle, err := windows.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("OpenProcess failed: %v", err))
	}

	a.debugMgr.mu.Lock()
	a.debugMgr.nextID++
	id := fmt.Sprintf("dbg-%d", a.debugMgr.nextID)
	a.debugMgr.mu.Unlock()

	session := &debugSession{
		id:     id,
		pid:    pid,
		handle: handle,
	}

	// Get module info for base address
	var modules [256]windows.Handle
	var needed uint32
	err = windows.EnumProcessModules(handle, &modules[0], uint32(len(modules)*int(unsafe.Sizeof(windows.Handle(0)))), &needed)
	if err == nil {
		var info windows.ModuleInfo
		err = windows.GetModuleInformation(handle, modules[0], &info, uint32(unsafe.Sizeof(info)))
		if err == nil {
			session.baseAddr = uint64(info.BaseOfDll)
		}
		var nameBuf [windows.MAX_PATH]uint16
		windows.GetModuleBaseName(handle, modules[0], &nameBuf[0], windows.MAX_PATH)
		session.name = windows.UTF16ToString(nameBuf[:])

		var pathBuf [windows.MAX_PATH]uint16
		windows.GetModuleFileNameEx(handle, modules[0], &pathBuf[0], windows.MAX_PATH)
		session.path = windows.UTF16ToString(pathBuf[:])
	}

	a.debugMgr.mu.Lock()
	a.debugMgr.sessions[id] = session
	a.debugMgr.mu.Unlock()

	return protocol.NewResult(env.ID, "debug_attached", map[string]interface{}{
		"debug_id":  id,
		"pid":       pid,
		"name":      session.name,
		"path":      session.path,
		"base_addr": session.baseAddr,
	})
}

// handleDebugReadMem reads memory from the attached process
func (a *Agent) handleDebugReadMem(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugReadMemParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.debugMgr.mu.Lock()
	session, ok := a.debugMgr.sessions[params.DebugID]
	a.debugMgr.mu.Unlock()
	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "debug session not found")
	}

	buf := make([]byte, params.Size)
	var nRead uintptr
	err := windows.ReadProcessMemory(session.handle, uintptr(params.Address), &buf[0], uintptr(params.Size), &nRead)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("ReadProcessMemory failed: %v (read %d bytes)", err, nRead))
	}

	data := buf[:nRead]
	b64 := base64.StdEncoding.EncodeToString(data)
	hexStr := hex.EncodeToString(data)

	return protocol.NewResult(env.ID, "debug_read_mem_result", map[string]interface{}{
		"data":     b64,
		"hex_data": hexStr,
		"size":     int(nRead),
		"address":  params.Address,
	})
}

// handleDebugModules lists loaded modules (DLLs) in the process
func (a *Agent) handleDebugModules(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugModulesParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.debugMgr.mu.Lock()
	session, ok := a.debugMgr.sessions[params.DebugID]
	a.debugMgr.mu.Unlock()
	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "debug session not found")
	}

	var modules [256]windows.Handle
	var needed uint32
	err := windows.EnumProcessModules(session.handle, &modules[0], uint32(len(modules)*int(unsafe.Sizeof(windows.Handle(0)))), &needed)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("EnumProcessModules failed: %v", err))
	}

	handleSize := int(unsafe.Sizeof(windows.Handle(0)))
	count := int(needed) / handleSize
	if count > len(modules) {
		count = len(modules)
	}

	var moduleList []protocol.DebugModuleInfo
	for i := 0; i < count; i++ {
		var info windows.ModuleInfo
		err := windows.GetModuleInformation(session.handle, modules[i], &info, uint32(unsafe.Sizeof(info)))
		if err != nil {
			continue
		}

		var nameBuf [windows.MAX_PATH]uint16
		windows.GetModuleBaseName(session.handle, modules[i], &nameBuf[0], windows.MAX_PATH)

		var pathBuf [windows.MAX_PATH]uint16
		windows.GetModuleFileNameEx(session.handle, modules[i], &pathBuf[0], windows.MAX_PATH)

		moduleList = append(moduleList, protocol.DebugModuleInfo{
			Name:     windows.UTF16ToString(nameBuf[:]),
			BaseAddr: uint64(info.BaseOfDll),
			Size:     int(info.SizeOfImage),
			Path:     windows.UTF16ToString(pathBuf[:]),
		})
	}

	return protocol.NewResult(env.ID, "debug_modules_result", map[string]interface{}{
		"modules": moduleList,
	})
}

// handleDebugMemQuery queries memory region info at an address
func (a *Agent) handleDebugMemQuery(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugMemQueryParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.debugMgr.mu.Lock()
	session, ok := a.debugMgr.sessions[params.DebugID]
	a.debugMgr.mu.Unlock()
	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "debug session not found")
	}

	var mbi windows.MemoryBasicInformation
	err := windows.VirtualQueryEx(session.handle, uintptr(params.Address), &mbi, unsafe.Sizeof(mbi))
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("VirtualQueryEx failed: %v", err))
	}

	return protocol.NewResult(env.ID, "debug_mem_query_result", map[string]interface{}{
		"region": protocol.DebugMemRegion{
			BaseAddress: uint64(mbi.BaseAddress),
			Size:        uint64(mbi.RegionSize),
			State:       uint32(mbi.State),
			Protect:     uint32(mbi.Protect),
			Type:        uint32(mbi.Type),
		},
	})
}

// handleDebugDetach closes the process handle
func (a *Agent) handleDebugDetach(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugDetachParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.debugMgr.mu.Lock()
	session, ok := a.debugMgr.sessions[params.DebugID]
	if ok {
		delete(a.debugMgr.sessions, params.DebugID)
	}
	a.debugMgr.mu.Unlock()

	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "debug session not found")
	}

	windows.CloseHandle(session.handle)

	return protocol.NewResult(env.ID, "debug_detached", map[string]interface{}{
		"detached": true,
		"debug_id": params.DebugID,
	})
}

func (a *Agent) closeAllDebug() {
	a.debugMgr.mu.Lock()
	defer a.debugMgr.mu.Unlock()
	for id, session := range a.debugMgr.sessions {
		windows.CloseHandle(session.handle)
		delete(a.debugMgr.sessions, id)
	}
}

// findProcessByName finds a PID by process name using Windows API
func findProcessByName(name string) (int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, &entry)
	if err != nil {
		return 0, err
	}

	for {
		procName := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(procName, name) {
			return int(entry.ProcessID), nil
		}
		err = windows.Process32Next(snapshot, &entry)
		if err != nil {
			break
		}
	}
	return 0, fmt.Errorf("process %s not found", name)
}