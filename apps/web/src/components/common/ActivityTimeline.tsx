import { cn } from "@/lib/utils";

interface Activity {
  id: string;
  time: string;
  title: string;
  description?: string;
}

interface ActivityTimelineProps {
  activities: Activity[];
  className?: string;
}

export function ActivityTimeline({ activities, className }: ActivityTimelineProps) {
  return (
    <div className={cn("space-y-4", className)}>
      {activities.map((activity, index) => (
        <div key={activity.id} className="flex gap-3">
          <div className="flex flex-col items-center">
            <div className="h-2 w-2 rounded-full bg-primary" />
            {index < activities.length - 1 && (
              <div className="mt-1 h-full w-px bg-border" />
            )}
          </div>
          <div className="pb-4">
            <p className="text-caption text-muted-foreground">{activity.time}</p>
            <p className="text-sm font-medium">{activity.title}</p>
            {activity.description && (
              <p className="text-caption text-muted-foreground">{activity.description}</p>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
