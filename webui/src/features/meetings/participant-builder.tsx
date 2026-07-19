import { useMemo } from "react"
import { DndContext, DragOverlay, PointerSensor, useDraggable, useDroppable, useSensor, useSensors } from "@dnd-kit/core"
import { GripVertical, Plus, Trash2 } from "lucide-react"
import type { ParticipantInput } from "@/lib/api"

const providers = {
  codex: { label: "Codex", model: "gpt-5.4-mini", models: ["gpt-5.4-mini"], option: "reasoning_effort", value: "medium" },
  opencode: { label: "OpenCode", model: "opencode-go/deepseek-v4-pro", models: ["opencode-go/deepseek-v4-pro"], option: "variant", value: "high" },
} as const

function ProviderCard({ provider }: { provider: keyof typeof providers }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: `provider:${provider}` })
  const item = providers[provider]
  return (
    <button
      className="provider-card"
      data-dragging={isDragging || undefined}
      ref={setNodeRef}
      style={transform ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` } : undefined}
      type="button"
      {...listeners}
      {...attributes}
    >
      <GripVertical aria-hidden="true" />
      <span><strong>{item.label}</strong><small>{item.model}</small></span>
    </button>
  )
}

function ParticipantRow({ item, recorderTaken, onChange, onRemove }: {
  item: ParticipantInput
  recorderTaken: boolean
  onChange: (next: ParticipantInput) => void
  onRemove: () => void
}) {
  const provider = providers[item.provider]
  return (
    <div className="participant-row">
      <span className="participant-provider">{provider.label}</span>
      <label><span>职责</span>
        <select value={item.role} onChange={(event) => {
          const role = event.target.value as ParticipantInput["role"]
          onChange({ ...item, role, seat: role === "recorder" ? "recorder" : `designer-${item.id.slice(0, 8)}` })
        }}>
          <option value="designer">Design Agent</option>
          <option disabled={recorderTaken && item.role !== "recorder"} value="recorder">Recorder</option>
        </select>
      </label>
      <label className="model-field"><span>模型</span>
        <input list={`models-${item.id}`} value={item.model} onChange={(event) => onChange({ ...item, model: event.target.value })} />
        <datalist id={`models-${item.id}`}>{provider.models.map((model) => <option key={model} value={model} />)}</datalist>
      </label>
      <label><span>{item.provider === "codex" ? "思考强度" : "Variant"}</span>
        <select
          value={item.provider_options[provider.option]}
          onChange={(event) => onChange({ ...item, provider_options: { [provider.option]: event.target.value } })}
        >
          {item.provider === "codex" && <option value="xhigh">xhigh</option>}
          <option value="high">high</option><option value="medium">medium</option><option value="low">low</option>
        </select>
      </label>
      <button aria-label={`移除 ${item.seat}`} className="icon-button" onClick={onRemove} type="button"><Trash2 /></button>
    </div>
  )
}

function ParticipantDropzone({ value, recorderCount, onChange }: {
  value: ParticipantInput[]
  recorderCount: number
  onChange: (next: ParticipantInput[]) => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id: "participant-dropzone" })
  return (
    <div className="participant-dropzone" data-over={isOver || undefined} ref={setNodeRef}>
      {value.length === 0 && <p>把 Provider 拖到这里。会议允许 0..N 个 Design Agent，但必须有且只有一个 Recorder。</p>}
      {value.map((item, index) => (
        <ParticipantRow
          item={item}
          key={item.id}
          recorderTaken={recorderCount > 0}
          onChange={(next) => onChange(value.map((candidate, candidateIndex) => candidateIndex === index ? next : candidate))}
          onRemove={() => onChange(value.filter((candidate) => candidate.id !== item.id))}
        />
      ))}
    </div>
  )
}

export function ParticipantBuilder({ value, onChange }: {
  value: ParticipantInput[]
  onChange: (next: ParticipantInput[]) => void
}) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const recorderCount = useMemo(() => value.filter((item) => item.role === "recorder").length, [value])

  function add(provider: keyof typeof providers) {
    const definition = providers[provider]
    const role = recorderCount === 0 ? "recorder" : "designer"
    const sequence = value.filter((item) => item.role === role).length + 1
    onChange([...value, {
      id: crypto.randomUUID(),
      seat: role === "recorder" ? "recorder" : `designer-${sequence}`,
      role,
      provider,
      model: definition.model,
      provider_options: { [definition.option]: definition.value },
    }])
  }

  return (
    <DndContext sensors={sensors} onDragEnd={({ active, over }) => {
      if (over?.id === "participant-dropzone") add(String(active.id).split(":")[1] as keyof typeof providers)
    }}>
      <div className="participant-builder">
        <div className="provider-palette">
          <div><span className="field-label">Provider</span><small>拖入会议，或点击添加</small></div>
          {(Object.keys(providers) as Array<keyof typeof providers>).map((provider) => (
            <div className="provider-choice" key={provider}>
              <ProviderCard provider={provider} />
              <button className="add-provider" onClick={() => add(provider)} type="button"><Plus />添加</button>
            </div>
          ))}
        </div>
        <ParticipantDropzone onChange={onChange} recorderCount={recorderCount} value={value} />
      </div>
      <DragOverlay><div className="drag-overlay">放入会议</div></DragOverlay>
    </DndContext>
  )
}
