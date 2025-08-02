import * as React from 'react';
import { cn } from '@/lib/utils';

const DropdownMenu = React.forwardRef<
    HTMLDivElement,
    React.ComponentProps<'div'> & {
        open?: boolean;
        onOpenChange?: (open: boolean) => void;
    }
>(({ className, open, onOpenChange, children, ...props }, ref) => {
    return (
        <div
            ref={ref}
            className={cn('relative inline-block', className)}
            {...props}
        >
            {children}
        </div>
    );
});
DropdownMenu.displayName = 'DropdownMenu';

const DropdownMenuTrigger = React.forwardRef<
    HTMLButtonElement,
    React.ComponentProps<'button'>
>(({ className, ...props }, ref) => {
    return (
        <button
            ref={ref}
            className={cn(
                'inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none',
                className
            )}
            {...props}
        />
    );
});
DropdownMenuTrigger.displayName = 'DropdownMenuTrigger';

const DropdownMenuContent = React.forwardRef<
    HTMLDivElement,
    React.ComponentProps<'div'> & {
        open?: boolean;
    }
>(({ className, open = false, children, ...props }, ref) => {
    if (!open) return null;

    return (
        <div
            ref={ref}
            className={cn(
                'absolute right-0 top-full z-50 min-w-[200px] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
                'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2',
                className
            )}
            {...props}
        >
            {children}
        </div>
    );
});
DropdownMenuContent.displayName = 'DropdownMenuContent';

const DropdownMenuItem = React.forwardRef<
    HTMLDivElement,
    React.ComponentProps<'div'> & {
        destructive?: boolean;
    }
>(({ className, destructive, ...props }, ref) => {
    return (
        <div
            ref={ref}
            className={cn(
                'relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
                destructive &&
                    'text-red-600 focus:bg-red-50 focus:text-red-600',
                className
            )}
            {...props}
        />
    );
});
DropdownMenuItem.displayName = 'DropdownMenuItem';

export {
    DropdownMenu,
    DropdownMenuTrigger,
    DropdownMenuContent,
    DropdownMenuItem,
};

