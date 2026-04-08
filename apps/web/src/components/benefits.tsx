import { Zap, Shield, Globe } from "lucide-react";

const benefits = [
  {
    icon: Zap,
    title: "Rápido",
    description: "Conversiones en segundos, sin esperas innecesarias.",
  },
  {
    icon: Shield,
    title: "Seguro",
    description: "Tus archivos se eliminan automáticamente tras el proceso.",
  },
  {
    icon: Globe,
    title: "Sin instalación",
    description: "Funciona directamente desde tu navegador, en cualquier dispositivo.",
  },
];

export default function Benefits() {
  return (
    <section className="mx-auto mt-12 grid max-w-xl gap-4 sm:grid-cols-3">
      {benefits.map((b) => {
        const Icon = b.icon;
        return (
          <div
            key={b.title}
            className="flex flex-col items-center gap-2 rounded-xl border border-sand-200 bg-white p-4 text-center"
          >
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-sand-100 text-sand-600">
              <Icon size={18} strokeWidth={2} aria-hidden="true" />
            </div>
            <p className="text-sm font-medium text-gray-800">{b.title}</p>
            <p className="text-xs leading-relaxed text-gray-400">
              {b.description}
            </p>
          </div>
        );
      })}
    </section>
  );
}
