"use client";

import { useRef, useState } from "react";
import { ImagePlus, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { api } from "@silieco/core/api";
import { useFileUpload } from "@silieco/core/hooks/use-file-upload";
import { EmojiPicker } from "@silieco/ui/components/common/emoji-picker";
import { Button } from "@silieco/ui/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@silieco/ui/components/ui/popover";
import { AvatarCropDialog } from "../../common/avatar-crop-dialog";
import { useT } from "../../i18n";
import { ProjectIcon } from "./project-icon";

interface ProjectIconPickerProps {
  value?: string | null;
  disabled?: boolean;
  onChange: (icon: string | null) => void | Promise<unknown>;
  onBusyChange?: (busy: boolean) => void;
}

/** Both create and edit share the same emoji and uploaded-image picker. */
export function ProjectIconPicker({ value, disabled, onChange, onBusyChange }: ProjectIconPickerProps) {
  const { t } = useT("projects");
  const { t: common } = useT("common");
  const { upload } = useFileUpload(api);
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);

  const commitChange = async (next: string | null) => {
    setBusy(true);
    onBusyChange?.(true);
    try {
      await onChange(next);
      setOpen(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.icon_picker.failed));
    } finally {
      setBusy(false);
      onBusyChange?.(false);
    }
  };

  const handleCropped = async (cropped: File) => {
    setBusy(true);
    onBusyChange?.(true);
    try {
      const result = await upload(cropped);
      if (!result) throw new Error(t(($) => $.icon_picker.failed));
      await onChange(result.link);
      setFile(null);
      toast.success(t(($) => $.icon_picker.updated));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.icon_picker.failed));
    } finally {
      setBusy(false);
      onBusyChange?.(false);
    }
  };

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <button
              type="button"
              disabled={disabled || busy}
              className="inline-flex size-12 -ml-1 items-center justify-center rounded-lg p-1 transition-colors hover:bg-accent/60 focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
              title={t(($) => $.detail.icon_tooltip)}
              aria-label={t(($) => $.detail.icon_tooltip)}
            >
              <ProjectIcon project={{ icon: value ?? null }} size="xl" />
            </button>
          }
        />
        <PopoverContent align="start" className="w-auto p-0">
          <div className="flex items-center gap-1 border-b border-border p-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="flex-1"
              disabled={busy}
              onClick={() => inputRef.current?.click()}
            >
              <ImagePlus className="size-4" />
              {t(($) => $.icon_picker.upload)}
            </Button>
            {value && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={busy}
                onClick={() => void commitChange(null)}
              >
                <RotateCcw className="size-4" />
                {t(($) => $.icon_picker.reset)}
              </Button>
            )}
          </div>
          <EmojiPicker onSelect={(emoji) => { if (!busy) void commitChange(emoji); }} />
        </PopoverContent>
      </Popover>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(event) => {
          const selected = event.target.files?.[0];
          event.target.value = "";
          if (!selected) return;
          if (!selected.type.startsWith("image/")) {
            toast.error(common(($) => $.avatar_upload.select_image));
            return;
          }
          setOpen(false);
          setFile(selected);
        }}
      />
      <AvatarCropDialog
        file={file}
        open={!!file}
        busy={busy}
        onOpenChange={(next) => { if (!next && !busy) setFile(null); }}
        onCropped={handleCropped}
      />
    </>
  );
}
