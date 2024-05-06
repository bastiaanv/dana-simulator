import { Refresh } from '@/assets/refresh.icon';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';

type FormProps = {
  name: string;
  type: '0' | '1' | '2';
  reservoir: number;
  battery: '25' | '50' | '75' | '100';
};

enum StatusEnum {
  LOADING,
  IDLE,
  RUNNING,
}

export function BasicInformationCard() {
  const { t } = useTranslation();

  const [status, setStatus] = useState<StatusEnum>(StatusEnum.IDLE);
  const form = useForm<FormProps>({ defaultValues: { name: '', type: '2', reservoir: 300, battery: '100' } });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('BASIC.TITLE')}</CardTitle>
        <CardDescription>{t('BASIC.SUB_TITLE')}</CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BASIC.FORM.NAME')}</FormLabel>
                <FormControl>
                  <div className="flex gap-3">
                    <Input {...field} disabled />
                    <Button disabled={status === StatusEnum.RUNNING}>
                      <Refresh width={20} height={20} fill="#fff" />
                    </Button>
                  </div>
                </FormControl>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="type"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BASIC.FORM.TYPE')}</FormLabel>
                <FormControl>
                  <Select onValueChange={field.onChange} defaultValue={field.value}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper">
                      <SelectItem value="0" disabled>
                        {t('BASIC.FORM.TYPES.0')}
                      </SelectItem>
                      <SelectItem value="1">{t('BASIC.FORM.TYPES.1')}</SelectItem>
                      <SelectItem value="2">{t('BASIC.FORM.TYPES.2')}</SelectItem>
                    </SelectContent>
                  </Select>
                </FormControl>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="reservoir"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BASIC.FORM.RESERVOIR')}</FormLabel>
                <FormControl>
                  <Input type="number" max={300} min={0} {...field} />
                </FormControl>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="battery"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BASIC.FORM.BATTERY')}</FormLabel>
                <FormControl>
                  <Select onValueChange={field.onChange} defaultValue={field.value}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper">
                      <SelectItem value="25">25%</SelectItem>
                      <SelectItem value="50">50%</SelectItem>
                      <SelectItem value="75">75%</SelectItem>
                      <SelectItem value="100">100%</SelectItem>
                    </SelectContent>
                  </Select>
                </FormControl>
              </FormItem>
            )}
          />

          <div className="flex gap-3 justify-end mt-6">
            <Button type="submit" disabled={!form.formState.isDirty}>
              {t('BASIC.ACTION.SAVE')}
            </Button>
            {status === StatusEnum.IDLE && <Button>{t('BASIC.ACTION.START')}</Button>}
            {status === StatusEnum.RUNNING && <Button>{t('BASIC.ACTION.STOP')}</Button>}
          </div>
        </Form>
      </CardContent>
    </Card>
  );
}
