"use client";

import { useModels, type LlemeModel } from "@/lib/lleme-api";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Circle } from "lucide-react";

interface ModelSelectorProps {
  value: string;
  onChange: (model: string) => void;
}

function getDisplayName(id: string) {
  const name = id.split("/").pop()?.split(":")[0] ?? id;
  return name.replace(/-GGUF$/, "");
}

function ModelOption({ model }: { model: LlemeModel }) {
  const isLoaded = model.lleme?.status === "ready";
  return (
    <span className="flex items-center gap-2">
      {isLoaded && <Circle className="size-2 fill-green-500 text-green-500" />}
      <span>{getDisplayName(model.id)}</span>
    </span>
  );
}

export function ModelSelector({ value, onChange }: ModelSelectorProps) {
  const { models, isLoading, error } = useModels();

  if (error) {
    return (
      <span className="text-sm text-destructive">Failed to load models</span>
    );
  }

  if (isLoading) {
    return <span className="text-sm text-muted-foreground">Loading...</span>;
  }

  const selectedModel = models.find((m) => m.id === value);

  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-9 w-auto gap-2 border-none bg-transparent px-2 shadow-none hover:bg-muted focus:ring-0">
        <SelectValue>
          {selectedModel && <ModelOption model={selectedModel} />}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {models.map((model) => (
          <SelectItem key={model.id} value={model.id}>
            <ModelOption model={model} />
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
