import { BGPattern } from "@/components/ui/bg-pattern";

export default function BGPatternDemo() {
    return (
        <div className="mx-auto max-w-4xl space-y-5 p-8">
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="grid" mask="fade-edges" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Grid Background</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-edges) Mask</p>
            </div>
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="dots" mask="fade-center" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Dots Background</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-center) Mask</p>
            </div>
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="diagonal-stripes" mask="fade-y" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Diagonal Stripes</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-y) Mask</p>
            </div>
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="horizontal-lines" mask="fade-right" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Horizontal Lines</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-right) Mask</p>
            </div>
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="vertical-lines" mask="fade-bottom" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Vertical Lines</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-bottom) Mask</p>
            </div>
            <div className="relative flex aspect-video flex-col items-center justify-center rounded-2xl border-2 border-[var(--border-strong)]">
                <BGPattern variant="checkerboard" mask="fade-top" fill="rgba(57,255,106,0.2)" />
                <h2 className="text-3xl font-bold text-[var(--primary-500)]">Checkerboard Background</h2>
                <p className="text-[var(--text-secondary)] font-mono">With (fade-top) Mask</p>
            </div>
        </div>
    );
}
