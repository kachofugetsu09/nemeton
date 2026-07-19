import { useMemo } from "react"
import { DndContext, DragOverlay, PointerSensor, useDraggable, useDroppable, useSensor, useSensors } from "@dnd-kit/core"
import { GripVertical, Plus, Trash2 } from "lucide-react"
import type { ParticipantInput, ProviderCatalog, ProviderDescriptor } from "@/lib/api"

function ProviderCard({ provider }: { provider: ProviderDescriptor }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: `provider:${provider.id}` })
  const defaultModel = provider.models.find((model) => model.default)
  if (!defaultModel) throw new Error(`Provider ${provider.id} does not define a default model`)
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
      <span><strong>{provider.label}</strong><small>{defaultModel.label}</small></span>
    </button>
  )
}

function ParticipantRow({ catalog, item, recorderTaken, onChange, onRemove }: {
  catalog: ProviderCatalog
  item: ParticipantInput
  recorderTaken: boolean
  onChange: (next: ParticipantInput) => void
  onRemove: () => void
}) {
  const provider = catalog.providers.find((candidate) => candidate.id === item.provider)
  if (!provider) throw new Error(`Provider catalog does not contain ${item.provider}`)
  const definition: ProviderDescriptor = provider
  const selectedModel = definition.models.find((model) => model.id === item.model)
  const optionValues = selectedModel?.option_values ?? definition.models[0].option_values

  function changeModel(modelID: string) {
    const model = definition.models.find((candidate) => candidate.id === modelID)
    if (definition.model_mode === "select" && !model) throw new Error(`Unknown ${definition.id} model ${modelID}`)
    const option = model?.default_option ?? item.provider_options[definition.option]
    onChange({ ...item, model: modelID, provider_options: { [definition.option]: option } })
  }

  return (
    <div className="participant-row">
      <span className="participant-provider">{definition.label}</span>
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
        {definition.model_mode === "select" ? (
          <select aria-label={`${item.seat} 模型`} value={item.model} onChange={(event) => changeModel(event.target.value)}>
            {definition.models.map((model) => <option key={model.id} value={model.id}>{model.label} · {model.id}</option>)}
          </select>
        ) : (
          <>
            <input aria-label={`${item.seat} 模型`} list={`models-${item.id}`} value={item.model} onChange={(event) => changeModel(event.target.value)} />
            <datalist id={`models-${item.id}`}>{definition.models.map((model) => <option key={model.id} value={model.id} />)}</datalist>
          </>
        )}
      </label>
      <label><span>{item.provider === "codex" ? "思考强度" : "Variant"}</span>
        <select
          aria-label={`${item.seat} ${definition.option}`}
          value={item.provider_options[definition.option]}
          onChange={(event) => onChange({ ...item, provider_options: { [definition.option]: event.target.value } })}
        >
          {optionValues.map((option) => <option key={option} value={option}>{option}</option>)}
        </select>
      </label>
      <button aria-label={`移除 ${item.seat}`} className="icon-button" onClick={onRemove} type="button"><Trash2 /></button>
    </div>
  )
}

function ParticipantDropzone({ catalog, value, recorderCount, onChange }: {
  catalog: ProviderCatalog
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
          catalog={catalog}
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

export function ParticipantBuilder({ catalog, value, onChange }: {
  catalog: ProviderCatalog
  value: ParticipantInput[]
  onChange: (next: ParticipantInput[]) => void
}) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const recorderCount = useMemo(() => value.filter((item) => item.role === "recorder").length, [value])

  function add(providerID: ParticipantInput["provider"]) {
    const provider = catalog.providers.find((candidate) => candidate.id === providerID)
    if (!provider) throw new Error(`Provider catalog does not contain ${providerID}`)
    const model = provider.models.find((candidate) => candidate.default)
    if (!model) throw new Error(`Provider ${providerID} does not define a default model`)
    const role = recorderCount === 0 ? "recorder" : "designer"
    const sequence = value.filter((item) => item.role === role).length + 1
    onChange([...value, {
      id: crypto.randomUUID(),
      seat: role === "recorder" ? "recorder" : `designer-${sequence}`,
      role,
      provider: providerID,
      model: model.id,
      provider_options: { [provider.option]: model.default_option },
    }])
  }

  return (
    <DndContext sensors={sensors} onDragEnd={({ active, over }) => {
      if (over?.id === "participant-dropzone") add(String(active.id).split(":")[1] as ParticipantInput["provider"])
    }}>
      <div className="participant-builder">
        <div className="provider-palette">
          <div><span className="field-label">Provider</span><small>拖入会议，或点击添加</small></div>
          {catalog.providers.map((provider) => (
            <div className="provider-choice" key={provider.id}>
              <ProviderCard provider={provider} />
              <button className="add-provider" onClick={() => add(provider.id)} type="button"><Plus />添加</button>
            </div>
          ))}
        </div>
        <ParticipantDropzone catalog={catalog} onChange={onChange} recorderCount={recorderCount} value={value} />
      </div>
      <DragOverlay><div className="drag-overlay">放入会议</div></DragOverlay>
    </DndContext>
  )
}
