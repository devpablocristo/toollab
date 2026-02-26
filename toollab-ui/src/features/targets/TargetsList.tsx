import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../lib/api";

export function TargetsList() {
  const deleteButtonClass =
    "bg-surface-raised border border-fail/30 text-fail px-4 py-2 rounded-xl font-semibold text-sm hover:bg-fail/10 transition-colors disabled:opacity-40";
  const qc = useQueryClient();
  const { data: targets, isLoading } = useQuery({
    queryKey: ["targets"],
    queryFn: api.listTargets,
  });

  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [desc, setDesc] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.createTarget({ name, base_url: baseUrl, description: desc }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["targets"] });
      setShowForm(false);
      setName("");
      setBaseUrl("");
      setDesc("");
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteTarget(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["targets"] }),
  });

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between animate-fade-in">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Targets</h1>
          <p className="text-text-secondary mt-1">Servicios bajo prueba</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-accent text-surface px-4 py-2.5 rounded-lg font-semibold text-sm hover:bg-accent-dim transition-colors"
          style={{ boxShadow: "0 0 20px rgba(0,232,157,0.15)" }}
        >
          + Agregar target
        </button>
      </div>

      <div className="divider-accent animate-fade-in" />

      {showForm && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
          className="lab-card p-6 space-y-4 animate-fade-in"
        >
          <div className="grid grid-cols-2 gap-4">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nombre (ej: nexus-core)"
              className="bg-surface border border-border-subtle rounded-lg px-4 py-2.5 text-sm outline-none transition-all"
              required
            />
            <input
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="Base URL (ej: http://localhost:8080)"
              className="bg-surface border border-border-subtle rounded-lg px-4 py-2.5 text-sm font-mono outline-none transition-all"
              required
            />
          </div>
          <input
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            placeholder="Descripci&oacute;n (opcional)"
            className="w-full bg-surface border border-border-subtle rounded-lg px-4 py-2.5 text-sm outline-none transition-all"
          />
          <div className="flex gap-3">
            <button
              type="submit"
              className="bg-accent text-surface px-5 py-2.5 rounded-lg font-semibold text-sm hover:bg-accent-dim transition-colors"
            >
              Crear
            </button>
            <button
              type="button"
              onClick={() => setShowForm(false)}
              className="text-text-secondary text-sm hover:text-text-primary transition-colors px-3"
            >
              Cancelar
            </button>
          </div>
        </form>
      )}

      {isLoading && (
        <div className="flex items-center gap-3 text-text-muted font-mono">
          <span className="w-1.5 h-1.5 rounded-full bg-accent animate-pulse" />
          Cargando&hellip;
        </div>
      )}

      {targets && targets.length === 0 && !showForm && (
        <div className="lab-card lab-card--neutral p-16 text-center animate-fade-in">
          <p className="text-text-muted font-mono text-lg">Sin targets</p>
          <p className="text-text-secondary text-sm mt-3">
            Agreg&aacute; un servicio para empezar
          </p>
        </div>
      )}

      {targets && targets.length > 0 && (
        <div className="space-y-3 animate-fade-in stagger-1">
          {targets.map((t) => (
            <div
              key={t.id}
              className="lab-card p-5 flex items-center justify-between"
            >
              <div>
                <p className="font-semibold text-text-primary">{t.name}</p>
                <p className="font-mono text-sm text-accent mt-0.5">
                  {t.base_url}
                </p>
                {t.description && (
                  <p className="text-text-secondary text-sm mt-1">
                    {t.description}
                  </p>
                )}
              </div>
              <button
                onClick={() => remove.mutate(t.id)}
                disabled={remove.isPending}
                className={deleteButtonClass}
              >
                Eliminar
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
